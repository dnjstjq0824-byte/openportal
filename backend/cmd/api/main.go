package main

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"example.com/openportal-search/backend/internal/config"
	"example.com/openportal-search/backend/internal/search"
)

type app struct {
	c          *search.Client
	corsOrigin string
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func (a *app) cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") == a.corsOrigin {
			w.Header().Set("Access-Control-Allow-Origin", a.corsOrigin)
			w.Header().Set("Vary", "Origin")
		}
		next(w, r)
	}
}
func validateQ(q string) error {
	if strings.TrimSpace(q) == "" {
		return errors.New("query is required")
	}
	if !utf8.ValidString(q) || utf8.RuneCountInString(q) > 200 {
		return errors.New("query must be valid UTF-8 and at most 200 characters")
	}
	return nil
}
func pageParam(s string) (int, error) {
	if s == "" {
		return 1, nil
	}
	p, err := strconv.Atoi(s)
	if err != nil || p < 1 || p > 100 {
		return 0, errors.New("page must be 1..100")
	}
	return p, nil
}
func (a *app) search(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if err := validateQ(q); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	page, err := pageParam(r.URL.Query().Get("page"))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	lang := r.URL.Query().Get("lang")
	if lang != "" && lang != "ko" && lang != "en" {
		writeJSON(w, 400, map[string]string{"error": "lang must be ko or en"})
		return
	}
	result, err := a.c.Search(r.Context(), q, lang, page, 10)
	if err != nil {
		log.Print("search backend failed")
		writeJSON(w, 503, map[string]string{"error": "search temporarily unavailable"})
		return
	}
	writeJSON(w, 200, result)
}
func (a *app) suggest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if err := validateQ(q); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	items, err := a.c.Suggest(r.Context(), q)
	if err != nil {
		log.Print("suggest backend failed")
		writeJSON(w, 503, map[string]string{"error": "suggest temporarily unavailable"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"suggestions": items})
}
func (a *app) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, 405, map[string]string{"error": "method not allowed"})
			return
		}
		if _, err := a.c.Search(r.Context(), "__readiness__", "", 1, 1); err != nil {
			writeJSON(w, 503, map[string]string{"status": "unavailable"})
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("/api/v1/search", a.cors(a.search))
	mux.HandleFunc("/api/v1/suggest", a.cors(a.suggest))
	return mux
}
func main() {
	cfg := config.Load()
	a := &app{c: search.New(cfg.OpenSearchURL, cfg.OpenSearchIndex, cfg.OpenSearchUsername, cfg.OpenSearchPassword), corsOrigin: cfg.CORSOrigin}
	srv := &http.Server{Addr: net.JoinHostPort(cfg.ListenHost, cfg.Port), Handler: a.routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	log.Printf("openportal search api listening port=%s", cfg.Port)
	log.Fatal(srv.ListenAndServe())
}
