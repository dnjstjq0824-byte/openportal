package config

import (
	"os"
	"strings"
)

type Config struct {
	ListenHost         string
	Port               string
	OpenSearchURL      string
	OpenSearchIndex    string
	OpenSearchUsername string
	OpenSearchPassword string
	CORSOrigin         string
	AuthDevMode        bool
	OIDCIssuer         string
	OIDCClientID       string
}

func env(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}
func Load() Config {
	return Config{
		ListenHost:         env("LISTEN_HOST", "127.0.0.1"),
		Port:               env("PORT", "8080"),
		OpenSearchURL:      strings.TrimRight(env("OPENSEARCH_URL", "http://localhost:9200"), "/"),
		OpenSearchIndex:    env("OPENSEARCH_INDEX", "documents_v1"),
		OpenSearchUsername: os.Getenv("OPENSEARCH_USERNAME"),
		OpenSearchPassword: os.Getenv("OPENSEARCH_PASSWORD"),
		CORSOrigin:         env("CORS_ORIGIN", "http://127.0.0.1:5173"),
		AuthDevMode:        strings.EqualFold(strings.TrimSpace(os.Getenv("AUTH_DEV_MODE")), "true"),
		OIDCIssuer:         strings.TrimSpace(os.Getenv("OIDC_ISSUER")),
		OIDCClientID:       strings.TrimSpace(os.Getenv("OIDC_CLIENT_ID")),
	}
}
