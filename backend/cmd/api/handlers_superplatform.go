package main

import (
	"net/http"
)

// A-6: the fabricated superplatform catalog was removed. No app backend exists,
// so the endpoint honestly reports not-implemented.
func (a *app) handleSuperplatformApps(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "superplatform apps backend not implemented", "code": "NOT_IMPLEMENTED"})
}

// A-6: the fabricated superplatform mailbox was removed. Mail lives behind the
// web gateway + JMAP provider; this endpoint has no backend and reports 501.

// A-6: the fabricated superplatform mailbox was removed. Mail lives behind the
// web gateway + JMAP provider; this endpoint has no backend and reports 501.
func (a *app) handleSuperplatformMail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "superplatform mail backend not implemented", "code": "NOT_IMPLEMENTED"})
}

// A-6: the fabricated developer-portal status (fake version, fake quota, fake
// SDK coordinates) was removed. No developer platform exists yet.

// A-6: the fabricated developer-portal status (fake version, fake quota, fake
// SDK coordinates) was removed. No developer platform exists yet.
func (a *app) handleSuperplatformDeveloper(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "superplatform developer backend not implemented", "code": "NOT_IMPLEMENTED"})
}
