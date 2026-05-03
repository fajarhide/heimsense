package router

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/fajarhide/heimsense/internal/adapter"
	"github.com/fajarhide/heimsense/internal/config"
)

func TestProviderChain_FallbackOn500(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Provider 1: Fails with 500 Internal Server Error
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv1.Close()

	// Provider 2: Succeeds
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := adapter.OpenAIResponse{
			ID:    "resp-2",
			Model: "gpt-4",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv2.Close()

	providers := []config.ProviderConfig{
		{Name: "p1", BaseURL: srv1.URL, MaxRetries: 0},
		{Name: "p2", BaseURL: srv2.URL, MaxRetries: 0},
	}

	chain := NewProviderChain(providers, 2*time.Second, logger)
	
	req := &adapter.OpenAIRequest{Model: "test"}
	resp, err := chain.ChatCompletion(context.Background(), req, "")
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if resp.ID != "resp-2" {
		t.Errorf("expected response from p2, got %s", resp.ID)
	}
}

func TestProviderChain_FallbackOn429(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Provider 1: Rate limited
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv1.Close()

	// Provider 2: Succeeds
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := adapter.OpenAIResponse{
			ID:    "resp-2",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv2.Close()

	providers := []config.ProviderConfig{
		{Name: "p1", BaseURL: srv1.URL, MaxRetries: 0},
		{Name: "p2", BaseURL: srv2.URL, MaxRetries: 0},
	}

	chain := NewProviderChain(providers, 2*time.Second, logger)
	
	req := &adapter.OpenAIRequest{Model: "test"}
	resp, err := chain.ChatCompletion(context.Background(), req, "")
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "resp-2" {
		t.Errorf("expected response from p2, got %s", resp.ID)
	}
}

func TestProviderChain_NoFallbackOn400(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Provider 1: Bad Request (Client Error)
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid model"))
	}))
	defer srv1.Close()

	// Provider 2: Succeeds (but shouldn't be reached)
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("provider 2 should not be called on a 400 error")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv2.Close()

	providers := []config.ProviderConfig{
		{Name: "p1", BaseURL: srv1.URL, MaxRetries: 0},
		{Name: "p2", BaseURL: srv2.URL, MaxRetries: 0},
	}

	chain := NewProviderChain(providers, 2*time.Second, logger)
	
	req := &adapter.OpenAIRequest{Model: "test"}
	_, err := chain.ChatCompletion(context.Background(), req, "")
	
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	
	if err == ErrAllProvidersFailed {
		t.Errorf("expected client error, got ErrAllProvidersFailed")
	}
}

func TestProviderChain_AllFailed(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv1.Close()

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv2.Close()

	providers := []config.ProviderConfig{
		{Name: "p1", BaseURL: srv1.URL, MaxRetries: 0},
		{Name: "p2", BaseURL: srv2.URL, MaxRetries: 0},
	}

	chain := NewProviderChain(providers, 2*time.Second, logger)
	
	req := &adapter.OpenAIRequest{Model: "test"}
	_, err := chain.ChatCompletion(context.Background(), req, "")
	
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
