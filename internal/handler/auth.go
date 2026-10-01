package handler

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/fajarhide/heimsense/internal/adapter"
)

// RequireProxyKey gates next behind a shared secret the caller has to present.
//
// An empty key makes this a pass-through, so a loopback-only install keeps
// working with no configuration. When a key is set the caller must send it as
// either "Authorization: Bearer <key>" or "x-api-key: <key>".
//
// On a match the credential is removed from the request. The upstream client
// forwards whatever Authorization it finds, and the proxy key is not a provider
// key, so leaving it in place would send it to the provider and fail there.
// Stripping it lets the request fall back to the operator's configured upstream
// key, which is the reason for having a gate at all.
func RequireProxyKey(key string, next http.Handler) http.Handler {
	if key == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !proxyKeyMatches(r, key) {
			writeAuthError(w)
			return
		}

		r.Header.Del("Authorization")
		r.Header.Del("X-Api-Key")
		next.ServeHTTP(w, r)
	})
}

func proxyKeyMatches(r *http.Request, key string) bool {
	presented := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if presented == "" {
		presented = strings.TrimSpace(r.Header.Get("X-Api-Key"))
	}

	// Compared in constant time so a wrong key leaks nothing through timing.
	return subtle.ConstantTimeCompare([]byte(presented), []byte(key)) == 1
}

func writeAuthError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(
		adapter.NewAnthropicError("authentication_error", "missing or invalid proxy credential"),
	)
}
