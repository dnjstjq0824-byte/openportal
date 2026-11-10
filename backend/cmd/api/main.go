package main

import (
	"context"
	"database/sql"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/dnjstjq0824-byte/openportal/backend/internal/config"
	"github.com/dnjstjq0824-byte/openportal/backend/internal/identity"
	"github.com/dnjstjq0824-byte/openportal/backend/internal/search"
	_ "github.com/lib/pq"
)

func main() {
	// B-5: request lines are self-contained JSON (with ts); drop the default
	// logger prefix so each line parses standalone.
	log.SetFlags(0)
	cfg := config.Load()
	var idStore *identity.Store
	if cfg.OIDCIssuer != "" && cfg.OIDCClientID != "" {
		verifier, err := identity.NewOIDCVerifier(context.Background(), cfg.OIDCIssuer, cfg.OIDCClientID)
		if err != nil {
			log.Printf("oidc verifier initialization failed: %v (auth endpoints fail-closed)", err)
		} else {
			idStore = identity.NewStoreWithVerifier(verifier)
		}
	}
	if idStore == nil {
		idStore = identity.NewStore()
	}
	// B-2: PostgreSQL is optional for the read-only search API but, when
	// configured, /readyz requires a live ping. The handle is lazy: an
	// unreachable database fails readiness, not boot (rolling-safe).
	var db *sql.DB
	if cfg.DatabaseURL != "" {
		handle, err := sql.Open("postgres", cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("invalid DATABASE_URL: %v", err)
		}
		handle.SetMaxOpenConns(4)
		handle.SetConnMaxLifetime(5 * time.Minute)
		db = handle
		defer db.Close()
	}
	a := &app{
		c:          search.New(cfg.OpenSearchURL, cfg.OpenSearchIndex, cfg.OpenSearchUsername, cfg.OpenSearchPassword),
		db:         db,
		idStore:    idStore,
		corsOrigin: cfg.CORSOrigin,
	}
	srv := &http.Server{Addr: net.JoinHostPort(cfg.ListenHost, cfg.Port), Handler: a.productionRoutes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	log.Printf("openportal search api listening port=%s", cfg.Port)
	log.Fatal(srv.ListenAndServe())
}
