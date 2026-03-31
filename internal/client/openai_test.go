package client

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fajarhide/heimsense/internal/adapter"
	"github.com/fajarhide/heimsense/internal/config"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ---------------------------------------------------------------------------
// New
// ---------------------------------------------------------------------------

func TestNew(t *testing.T) {
	cfg := &config.Config{
		UpstreamBaseURL: "https://example.com/v1",
		APIKey:          "sk-test",
		RequestTimeout:  30 * time.Second,
		MaxRetries:      5,
	}
	c := New(cfg, testLogger())

	if c.baseURL != cfg.UpstreamBaseURL {
		t.Errorf("baseURL = %q, want %q", c.baseURL, cfg.UpstreamBaseURL)
	}
	if c.apiKey != cfg.APIKey {
		t.Errorf("apiKey = %q, want %q", c.apiKey, cfg.APIKey)
	}
	if c.maxRetries != cfg.MaxRetries {
		t.Errorf("maxRetries = %d, want %d", c.maxRetries, cfg.MaxRetries)
	}
	if c.httpClient.Timeout != cfg.RequestTimeout {
		t.Errorf("httpClient.Timeout = %v, want %v", c.httpClient.Timeout, cfg.RequestTimeout)
	}
}

// ---------------------------------------------------------------------------
// ChatCompletion — success
// ---------------------------------------------------------------------------

func TestChatCompletion_Success(t *testing.T) {
	oaiResp := adapter.OpenAIResponse{
		ID:    "chatcmpl-123",
		Model: "gpt-4",
		Choices: []adapter.OpenAIChoice{
			{
				Index:   0,
				Message: adapter.OpenAIMessage{Role: "assistant", Content: "hello"},
			},
		},
		Usage: &adapter.OpenAIUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify path
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		// Verify method
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %q", r.Method)
		}
		// Verify content-type
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(oaiResp)
	}))
	defer srv.Close()

	c := &Client{
		httpClient: srv.Client(),
		baseURL:    srv.URL,
		apiKey:     "sk-test",
		maxRetries: 0,
		logger:     testLogger(),
	}

	req := &adapter.OpenAIRequest{
		Model:    "gpt-4",
		Messages: []adapter.OpenAIMessage{{Role: "user", Content: "hi"}},
		Stream:   true, // should be set to false by ChatCompletion
	}

	resp, err := c.ChatCompletion(context.Background(), req, "")
	if err != nil {
		t.Fatalf("ChatCompletion() error: %v", err)
	}

	// Verify stream was forced to false
	if req.Stream != false {
		t.Error("ChatCompletion should set Stream=false")
	}

	if resp.ID != oaiResp.ID {
		t.Errorf("resp.ID = %q, want %q", resp.ID, oaiResp.ID)
	}
	if resp.Model != oaiResp.Model {
		t.Errorf("resp.Model = %q, want %q", resp.Model, oaiResp.Model)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("resp.Choices length = %d, want 1", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "hello" {
		t.Errorf("resp.Choices[0].Message.Content = %q, want %q", resp.Choices[0].Message.Content, "hello")
	}
	if resp.Usage == nil {
		t.Fatal("resp.Usage is nil")
	}
	if resp.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", resp.Usage.PromptTokens)
	}
}

// ---------------------------------------------------------------------------
// ChatCompletion — upstream returns invalid JSON
// ---------------------------------------------------------------------------

func TestChatCompletion_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not valid json`))
	}))
	defer srv.Close()

	c := &Client{
		httpClient: srv.Client(),
		baseURL:    srv.URL,
		apiKey:     "",
		maxRetries: 0,
		logger:     testLogger(),
	}

	_, err := c.ChatCompletion(context.Background(), &adapter.OpenAIRequest{
		Messages: []adapter.OpenAIMessage{{Role: "user", Content: "hi"}},
	}, "")

	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "unmarshal response") {
		t.Errorf("error = %q, want it to contain 'unmarshal response'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// ChatCompletionStream — success
// ---------------------------------------------------------------------------

func TestChatCompletionStream_Success(t *testing.T) {
	body := "data: {\"id\":\"chatcmpl-1\"}\n\ndata: [DONE]\n\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	c := &Client{
		httpClient: srv.Client(),
		baseURL:    srv.URL,
		apiKey:     "sk-test",
		maxRetries: 0,
		logger:     testLogger(),
	}

	req := &adapter.OpenAIRequest{
		Messages: []adapter.OpenAIMessage{{Role: "user", Content: "hi"}},
		Stream:   false,
	}

	rc, err := c.ChatCompletionStream(context.Background(), req, "")
	if err != nil {
		t.Fatalf("ChatCompletionStream() error: %v", err)
	}
	defer rc.Close()

	// Verify stream was forced to true
	if req.Stream != true {
		t.Error("ChatCompletionStream should set Stream=true")
	}

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}
	if string(data) != body {
		t.Errorf("body = %q, want %q", string(data), body)
	}
}

// ---------------------------------------------------------------------------
// doWithRetry — auth header passthrough
// ---------------------------------------------------------------------------

func TestDoWithRetry_AuthHeaderPassthrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer custom-key" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer custom-key")
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := &Client{
		httpClient: srv.Client(),
		baseURL:    srv.URL,
		apiKey:     "sk-default",
		maxRetries: 0,
		logger:     testLogger(),
	}

	rc, err := c.doWithRetry(context.Background(), []byte(`{}`), "Bearer custom-key")
	if err != nil {
		t.Fatal(err)
	}
	rc.Close()
}

func TestDoWithRetry_FallbackAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer sk-default" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer sk-default")
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := &Client{
		httpClient: srv.Client(),
		baseURL:    srv.URL,
		apiKey:     "sk-default",
		maxRetries: 0,
		logger:     testLogger(),
	}

	rc, err := c.doWithRetry(context.Background(), []byte(`{}`), "")
	if err != nil {
		t.Fatal(err)
	}
	rc.Close()
}

// ---------------------------------------------------------------------------
// doWithRetry — 5xx retries
// ---------------------------------------------------------------------------

func TestDoWithRetry_Retries5xx(t *testing.T) {
	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"server error"}`))
			return
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := &Client{
		httpClient: srv.Client(),
		baseURL:    srv.URL,
		apiKey:     "",
		maxRetries: 3,
		logger:     testLogger(),
	}

	rc, err := c.doWithRetry(context.Background(), []byte(`{}`), "")
	if err != nil {
		t.Fatalf("expected success after retries, got error: %v", err)
	}
	rc.Close()

	if atomic.LoadInt32(&calls) != 3 {
		t.Errorf("expected 3 calls (2 retries + success), got %d", atomic.LoadInt32(&calls))
	}
}

// ---------------------------------------------------------------------------
// doWithRetry — 4xx no retry
// ---------------------------------------------------------------------------

func TestDoWithRetry_NoRetry4xx(t *testing.T) {
	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer srv.Close()

	c := &Client{
		httpClient: srv.Client(),
		baseURL:    srv.URL,
		apiKey:     "",
		maxRetries: 3,
		logger:     testLogger(),
	}

	_, err := c.doWithRetry(context.Background(), []byte(`{}`), "")
	if err == nil {
		t.Fatal("expected error for 400 response")
	}

	if !strings.Contains(err.Error(), "upstream 400") {
		t.Errorf("error = %q, should contain 'upstream 400'", err.Error())
	}

	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("expected 1 call (no retries for 4xx), got %d", atomic.LoadInt32(&calls))
	}
}

// ---------------------------------------------------------------------------
// doWithRetry — all retries exhausted
// ---------------------------------------------------------------------------

func TestDoWithRetry_AllRetriesExhausted(t *testing.T) {
	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`service unavailable`))
	}))
	defer srv.Close()

	c := &Client{
		httpClient: srv.Client(),
		baseURL:    srv.URL,
		apiKey:     "",
		maxRetries: 2,
		logger:     testLogger(),
	}

	_, err := c.doWithRetry(context.Background(), []byte(`{}`), "")
	if err == nil {
		t.Fatal("expected error when all retries exhausted")
	}
	if !strings.Contains(err.Error(), "retries exhausted") {
		t.Errorf("error = %q, want 'retries exhausted'", err.Error())
	}

	// 1 initial + 2 retries = 3 total
	if atomic.LoadInt32(&calls) != 3 {
		t.Errorf("expected 3 calls, got %d", atomic.LoadInt32(&calls))
	}
}

// ---------------------------------------------------------------------------
// doWithRetry — context cancellation during backoff
// ---------------------------------------------------------------------------

func TestDoWithRetry_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`error`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())

	c := &Client{
		httpClient: srv.Client(),
		baseURL:    srv.URL,
		apiKey:     "",
		maxRetries: 10,
		logger:     testLogger(),
	}

	// Cancel immediately after first attempt would trigger a backoff
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	_, err := c.doWithRetry(ctx, []byte(`{}`), "")
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
}

// ---------------------------------------------------------------------------
// truncate
// ---------------------------------------------------------------------------

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"needs truncation", "hello world", 5, "hello..."},
		{"empty string", "", 5, ""},
		{"zero max", "hello", 0, "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}
