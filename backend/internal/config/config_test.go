package config

import "testing"

func TestLoadConfigDefaultsAndOverrides(t *testing.T) {
	t.Setenv("LISTEN_HOST", "")
	t.Setenv("PORT", "")
	t.Setenv("OPENSEARCH_URL", "")
	t.Setenv("OPENSEARCH_INDEX", "")
	t.Setenv("CORS_ORIGIN", "")

	cfg := Load()
	if cfg.ListenHost != "127.0.0.1" || cfg.Port != "8080" || cfg.OpenSearchURL != "http://localhost:9200" || cfg.OpenSearchIndex != "documents_v2" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}

	t.Setenv("OPENSEARCH_URL", "http://127.0.0.1:9200/")
	t.Setenv("PORT", "9090")
	cfg2 := Load()
	if cfg2.OpenSearchURL != "http://127.0.0.1:9200" || cfg2.Port != "9090" {
		t.Fatalf("unexpected overrides: %+v", cfg2)
	}
}

