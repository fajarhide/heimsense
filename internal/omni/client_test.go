package omni

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestClient_Distill_Success(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/distill" {
			t.Errorf("expected /distill, got %s", r.URL.Path)
		}
		
		var req DistillRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		
		if req.Content != "large diff content" {
			t.Errorf("unexpected content: %s", req.Content)
		}
		
		resp := DistillResponse{
			Distilled:       "small diff",
			OriginalTokens:  100,
			DistilledTokens: 20,
			FiltersApplied:  []string{"git-diff"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, logger)
	
	distilled, stats, err := client.Distill(context.Background(), "large diff content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if distilled != "small diff" {
		t.Errorf("expected 'small diff', got '%s'", distilled)
	}
	
	if stats == nil {
		t.Fatal("expected stats, got nil")
	}
	if stats.OriginalTokens != 100 || stats.DistilledTokens != 20 {
		t.Errorf("unexpected token counts: %d -> %d", stats.OriginalTokens, stats.DistilledTokens)
	}
	if stats.SavedPercent != 80.0 {
		t.Errorf("expected 80.0 saved percent, got %f", stats.SavedPercent)
	}
}

func TestClient_Distill_ErrorFallback(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	
	// Server returns 500 error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := NewClient(server.URL, logger)
	
	distilled, stats, err := client.Distill(context.Background(), "original content")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	
	// Should fallback to returning the original content
	if distilled != "original content" {
		t.Errorf("expected fallback to original content, got '%s'", distilled)
	}
	
	if stats != nil {
		t.Errorf("expected nil stats on error, got %v", stats)
	}
}

func TestClient_IsHealthy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("expected /health, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, logger)
	if !client.IsHealthy(context.Background()) {
		t.Error("expected IsHealthy to be true")
	}
}

func TestClient_IsHealthy_Failed(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewClient(server.URL, logger)
	if client.IsHealthy(context.Background()) {
		t.Error("expected IsHealthy to be false")
	}
}
