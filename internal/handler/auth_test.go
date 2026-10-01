package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// echoAuth reports back what the wrapped handler actually received, so a test
// can prove the proxy credential was stripped rather than forwarded upstream.
func echoAuth(seen *string, called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		*seen = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	})
}

func TestRequireProxyKey_EmptyKeyIsPassThrough(t *testing.T) {
	var called bool
	var seen string

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	req.Header.Set("Authorization", "Bearer sk-upstream")
	rec := httptest.NewRecorder()

	RequireProxyKey("", echoAuth(&seen, &called)).ServeHTTP(rec, req)

	if !called {
		t.Fatal("handler was not reached with no proxy key configured")
	}
	if seen != "Bearer sk-upstream" {
		t.Errorf("Authorization = %q, want it forwarded untouched", seen)
	}
}

func TestRequireProxyKey_RejectsMissingAndWrongCredential(t *testing.T) {
	for name, header := range map[string]map[string]string{
		"no header":    {},
		"wrong bearer": {"Authorization": "Bearer nope"},
		"wrong apikey": {"X-Api-Key": "nope"},
		"empty bearer": {"Authorization": "Bearer "},
	} {
		t.Run(name, func(t *testing.T) {
			var called bool
			var seen string

			req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			for k, v := range header {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()

			RequireProxyKey("secret", echoAuth(&seen, &called)).ServeHTTP(rec, req)

			if called {
				t.Error("handler was reached without a valid credential")
			}
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestRequireProxyKey_AcceptsAndStripsCredential(t *testing.T) {
	for name, header := range map[string]map[string]string{
		"bearer": {"Authorization": "Bearer secret"},
		"apikey": {"X-Api-Key": "secret"},
		"raw":    {"Authorization": "secret"},
	} {
		t.Run(name, func(t *testing.T) {
			var called bool
			var seen string

			req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			for k, v := range header {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()

			RequireProxyKey("secret", echoAuth(&seen, &called)).ServeHTTP(rec, req)

			if !called {
				t.Fatalf("handler not reached, status %d", rec.Code)
			}
			// The proxy key is not a provider key, so forwarding it upstream
			// would fail at the provider.
			if seen != "" {
				t.Errorf("Authorization = %q, want it stripped before upstream", seen)
			}
		})
	}
}
