package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fajarhide/heimsense/internal/client"
	"github.com/fajarhide/heimsense/internal/config"
	"github.com/fajarhide/heimsense/internal/usage"
)

func TestMessagesHandler_RefusesOverCeiling(t *testing.T) {
	cfg := &config.Config{
		DefaultModel:         "gpt-4",
		UsageWindow:          time.Hour,
		MaxRequestsPerWindow: 1,
	}
	h := NewMessagesHandler(client.New(cfg, testLogger()), cfg, testLogger())

	// Spend the single allowed request, then the next one must be refused
	// before any upstream call is attempted.
	if err := h.Meter().Allow(); err != nil {
		t.Fatalf("Allow() = %v, want nil", err)
	}

	body := `{"model":"claude-sonnet-4","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After header is missing, a caller cannot back off sensibly")
	}
	if !strings.Contains(rec.Body.String(), "rate_limit_error") {
		t.Errorf("body = %s, want an Anthropic rate_limit_error", rec.Body.String())
	}
}

func TestUsageHandler_ReportsCounters(t *testing.T) {
	meter := usage.New(usage.Limits{Window: time.Hour, MaxTokens: 1000, MaxRequests: 10})
	_ = meter.Allow()
	meter.AddTokens(30, 12)

	req := httptest.NewRequest(http.MethodGet, "/usage", nil)
	rec := httptest.NewRecorder()

	NewUsageHandler(meter).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var snap usage.Snapshot
	if err := json.NewDecoder(rec.Body).Decode(&snap); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if snap.Requests != 1 {
		t.Errorf("requests = %d, want 1", snap.Requests)
	}
	if snap.Tokens != 42 {
		t.Errorf("tokens = %d, want 42", snap.Tokens)
	}
	if snap.MaxTokens != 1000 || snap.MaxRequests != 10 {
		t.Errorf("ceilings = %d/%d, want 1000/10", snap.MaxTokens, snap.MaxRequests)
	}
}

func TestUsageHandler_RejectsNonGet(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/usage", nil)
	rec := httptest.NewRecorder()

	NewUsageHandler(usage.New(usage.Limits{})).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
