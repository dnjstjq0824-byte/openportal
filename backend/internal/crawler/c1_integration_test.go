//go:build integration

// C-1 integration proof: the DB-backed frontier (crawl.hosts/frontier) against
// live PostgreSQL. Run explicitly:
//
//	TEST_PG_URL='postgres://...' go test -count=1 -tags integration -run TestC1 ./internal/crawler/
package crawler

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func c1db(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_PG_URL")
	if dsn == "" {
		t.Skip("TEST_PG_URL not set; run against the compose postgres")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	return db
}

func TestC1PostgresFrontierCycle(t *testing.T) {
	db := c1db(t)
	defer db.Close()
	const host = "c1test.example.test"
	const page = "https://c1test.example.test/probe-page"
	_, _ = db.Exec(`DELETE FROM crawl.frontier WHERE url = $1`, page)
	_, _ = db.Exec(`DELETE FROM crawl.hosts WHERE hostname = $1`, host)

	frontier := NewPostgresFrontier(db)
	frontier.RegisterVerifiedHost(&VerifiedHost{
		ID: "c1-host", Scheme: "https", Hostname: host, Port: 443,
		HostKey: "https://c1test.example.test:443", FetchEnabled: true,
		CrawlDelay: 100 * time.Millisecond, CreatedAt: time.Now().UTC(),
	})
	var hostCount int
	if err := db.QueryRow(`SELECT count(*) FROM crawl.hosts WHERE hostname=$1`, host).Scan(&hostCount); err != nil || hostCount != 1 {
		t.Fatalf("host row missing: count=%d err=%v", hostCount, err)
	}
	submitted, err := frontier.SubmitURL(page, 80)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	leased := frontier.LeaseAcquire("c1-worker", 5, time.Minute)
	if len(leased) != 1 || leased[0].URLHash != submitted.URLHash {
		t.Fatalf("leased=%v", leased)
	}
	if err := frontier.Ack(submitted.URLHash, "c1-worker"); err != nil {
		t.Fatalf("ack: %v", err)
	}
	// Age the completion, then prove the freshness scheduler re-queues it.
	if _, err := db.Exec(`UPDATE crawl.frontier SET scheduled_at = now() - interval '8 days' WHERE url_hash = $1`, submitted.URLHash); err != nil {
		t.Fatalf("age: %v", err)
	}
	n, err := frontier.RescheduleStale(time.Now().UTC().Add(-7*24*time.Hour), 100)
	if err != nil || n != 1 {
		t.Fatalf("reschedule n=%d err=%v", n, err)
	}
	released := frontier.LeaseAcquire("c1-worker", 5, time.Minute)
	if len(released) != 1 {
		t.Fatalf("re-queued row must lease again, got %d", len(released))
	}
	_, _ = db.Exec(`DELETE FROM crawl.frontier WHERE url = $1`, page)
	_, _ = db.Exec(`DELETE FROM crawl.hosts WHERE hostname = $1`, host)
}
