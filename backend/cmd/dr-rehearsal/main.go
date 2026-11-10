// Command dr-rehearsal probes disaster-recovery readiness signals against live
// infrastructure: PostgreSQL reachability + current WAL LSN, and OpenSearch
// reachability + cluster health. It replaces the previous fake rehearsal that
// hashed a hardcoded "WAL LSN" string and slept (A-5).
//
// Scope is explicit: this is a READINESS PROBE, not a full PITR cluster
// rebuild. It does not measure true restore RPO and must not be cited as a
// full DR rehearsal. Exit code is 1 when any target is missing or unreachable.
//
// Usage:
//
//	DATABASE_URL=postgres://... OPENSEARCH_URL=http://127.0.0.1:9200 go run ./cmd/dr-rehearsal
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/dnjstjq0824-byte/openportal/backend/internal/launch"
	_ "github.com/lib/pq"
)

func fail(format string, args ...any) int {
	fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
	return 1
}

func main() {
	os.Exit(run())
}

func run() int {
	databaseURL := flag.String("database-url", os.Getenv("DATABASE_URL"), "Live PostgreSQL URL (or $DATABASE_URL)")
	opensearchURL := flag.String("opensearch-url", os.Getenv("OPENSEARCH_URL"), "Live OpenSearch base URL (or $OPENSEARCH_URL)")
	flag.Parse()

	if *databaseURL == "" {
		return fail("no PostgreSQL target: pass -database-url or set DATABASE_URL")
	}
	if *opensearchURL == "" {
		return fail("no OpenSearch target: pass -opensearch-url or set OPENSEARCH_URL")
	}

	fmt.Println("==================================================================")
	fmt.Println(" OpenPortal DR Readiness Probe (live infrastructure only)")
	fmt.Println(" Scope: connectivity + WAL LSN + cluster health. NOT a full PITR rebuild.")
	fmt.Println("==================================================================")

	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Step 1: live PostgreSQL roundtrip with the real current WAL LSN.
	fmt.Println("[Step 1/3] Probing PostgreSQL (ping + version + current WAL LSN)...")
	db, err := sql.Open("postgres", *databaseURL)
	if err != nil {
		return fail("postgres open failed: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return fail("postgres unreachable: %v", err)
	}
	var pgVersion, pgLSN string
	if err := db.QueryRowContext(ctx, "SELECT version()").Scan(&pgVersion); err != nil {
		return fail("postgres version query failed: %v", err)
	}
	if err := db.QueryRowContext(ctx, "SELECT pg_current_wal_lsn()::text").Scan(&pgLSN); err != nil {
		return fail("postgres WAL LSN query failed: %v", err)
	}
	standbyLagSec := 0
	standbyNote := "no standby configured (pg_stat_replication empty)"
	var lag sql.NullFloat64
	if err := db.QueryRowContext(ctx, "SELECT max(EXTRACT(EPOCH FROM replay_lag)) FROM pg_stat_replication").Scan(&lag); err != nil {
		return fail("postgres replication-lag query failed: %v", err)
	}
	if lag.Valid {
		standbyLagSec = int(lag.Float64)
		standbyNote = fmt.Sprintf("max standby replay lag %.0fs", lag.Float64)
	}
	fmt.Printf(" -> PostgreSQL OK: %s\n", truncate(pgVersion, 80))
	fmt.Printf(" -> Current WAL LSN: %s | %s\n", pgLSN, standbyNote)

	// Step 2: live OpenSearch roundtrip (root info + cluster health).
	fmt.Println("[Step 2/3] Probing OpenSearch (node info + cluster health)...")
	httpClient := &http.Client{Timeout: 15 * time.Second}
	rootBody, err := httpGet(ctx, httpClient, *opensearchURL+"/")
	if err != nil {
		return fail("opensearch unreachable: %v", err)
	}
	var rootInfo struct {
		Version struct {
			Number string `json:"number"`
		} `json:"version"`
		Tagline string `json:"tagline"`
	}
	if err := json.Unmarshal(rootBody, &rootInfo); err != nil {
		return fail("opensearch root response is not valid JSON: %v", err)
	}
	healthBody, err := httpGet(ctx, httpClient, *opensearchURL+"/_cluster/health")
	if err != nil {
		return fail("opensearch cluster health unreachable: %v", err)
	}
	var health struct {
		Status              string `json:"status"`
		NumberOfNodes       int    `json:"number_of_nodes"`
		UnassignedShards    int    `json:"unassigned_shards"`
		ActivePrimaryShards int    `json:"active_primary_shards"`
	}
	if err := json.Unmarshal(healthBody, &health); err != nil {
		return fail("opensearch cluster health is not valid JSON: %v", err)
	}
	fmt.Printf(" -> OpenSearch OK: version %s (%s)\n", rootInfo.Version.Number, truncate(rootInfo.Tagline, 60))
	fmt.Printf(" -> Cluster health: status=%s nodes=%d active_primary=%d unassigned=%d\n",
		health.Status, health.NumberOfNodes, health.ActivePrimaryShards, health.UnassignedShards)
	if health.Status == "red" {
		return fail("opensearch cluster status is red")
	}

	// Step 3: record the probe with REAL measured values only.
	fmt.Println("[Step 3/3] Recording readiness-probe evidence...")
	evidence, err := json.Marshal(map[string]any{
		"probe":              "dr_readiness_probe",
		"postgres_version":   pgVersion,
		"postgres_wal_lsn":   pgLSN,
		"standby_lag_sec":    standbyLagSec,
		"opensearch_version": rootInfo.Version.Number,
		"cluster_status":     health.Status,
		"cluster_nodes":      health.NumberOfNodes,
		"probed_at":          time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return fail("cannot marshal probe evidence: %v", err)
	}
	drMgr := launch.NewDRManager()
	elapsedSec := int(time.Since(started).Seconds())
	drill, err := drMgr.RecordDrill(
		fmt.Sprintf("dr_readiness_probe_%d", time.Now().Unix()),
		1800,          // target RTO (documented SLO)
		elapsedSec,    // actual: measured probe time (NOT a full restore)
		300,           // target RPO (documented SLO)
		standbyLagSec, // actual: measured standby replay lag only
		evidence,
		"dr_readiness_probe_runner",
	)
	if err != nil {
		return fail("cannot record drill: %v", err)
	}
	record, _ := json.MarshalIndent(drill, "", "  ")
	fmt.Printf(" -> Drill record:\n%s\n", record)
	fmt.Println("------------------------------------------------------------------")
	fmt.Println(" Scope reminder: readiness probe passed. A full PITR restore")
	fmt.Println(" rehearsal (rebuild + measured RPO) was NOT performed.")
	fmt.Println("------------------------------------------------------------------")
	return 0
}

func httpGet(ctx context.Context, client *http.Client, target string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, target)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
