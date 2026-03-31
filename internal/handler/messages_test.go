package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fajarhide/heimsense/internal/adapter"
	"github.com/fajarhide/heimsense/internal/client"
	"github.com/fajarhide/heimsense/internal/config"
)

func testCfg() *config.Config {
	return &config.Config{
		DefaultModel: "gpt-4",
		ForceModel:   "",
		MaxRetries:   0,
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ---------------------------------------------------------------------------
// HealthHandler
// ---------------------------------------------------------------------------

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	HealthHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	expected := `{"status":"ok"}`
	if rec.Body.String() != expected {
		t.Errorf("expected body %q, got %q", expected, rec.Body.String())
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// ---------------------------------------------------------------------------
// ServeHTTP — method not allowed
// ---------------------------------------------------------------------------

func TestServeHTTP_MethodNotAllowed(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, m := range methods {
		t.Run(m, func(t *testing.T) {
			cfg := testCfg()
			// Create a dummy upstream that should never be called
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				t.Error("upstream should not be called")
			}))
			defer upstream.Close()

			cfg.UpstreamBaseURL = upstream.URL
			c := client.New(cfg, testLogger())
			h := NewMessagesHandler(c, cfg, testLogger())

			req := httptest.NewRequest(m, "/v1/messages", nil)
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
			}

			var errResp adapter.AnthropicError
			if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode error: %v", err)
			}
			if errResp.Error.Type != "invalid_request_error" {
				t.Errorf("error type = %q, want invalid_request_error", errResp.Error.Type)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ServeHTTP — invalid JSON body
// ---------------------------------------------------------------------------

func TestServeHTTP_InvalidJSON(t *testing.T) {
	cfg := testCfg()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("upstream should not be called for invalid JSON")
	}))
	defer upstream.Close()

	cfg.UpstreamBaseURL = upstream.URL
	c := client.New(cfg, testLogger())
	h := NewMessagesHandler(c, cfg, testLogger())

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader("not json"))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var errResp adapter.AnthropicError
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error: %v", err)
	}
	if !strings.Contains(errResp.Error.Message, "invalid JSON") {
		t.Errorf("error message = %q, want containing 'invalid JSON'", errResp.Error.Message)
	}
}

// ---------------------------------------------------------------------------
// ServeHTTP — missing messages
// ---------------------------------------------------------------------------

func TestServeHTTP_MissingMessages(t *testing.T) {
	cfg := testCfg()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("upstream should not be called")
	}))
	defer upstream.Close()

	cfg.UpstreamBaseURL = upstream.URL
	c := client.New(cfg, testLogger())
	h := NewMessagesHandler(c, cfg, testLogger())

	body := `{"model":"claude-3","max_tokens":100,"messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var errResp adapter.AnthropicError
	json.NewDecoder(rec.Body).Decode(&errResp)
	if !strings.Contains(errResp.Error.Message, "messages is required") {
		t.Errorf("error message = %q, want 'messages is required'", errResp.Error.Message)
	}
}

// ---------------------------------------------------------------------------
// ServeHTTP — missing max_tokens
// ---------------------------------------------------------------------------

func TestServeHTTP_MissingMaxTokens(t *testing.T) {
	cfg := testCfg()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("upstream should not be called")
	}))
	defer upstream.Close()

	cfg.UpstreamBaseURL = upstream.URL
	c := client.New(cfg, testLogger())
	h := NewMessagesHandler(c, cfg, testLogger())

	body := `{"model":"claude-3","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var errResp adapter.AnthropicError
	json.NewDecoder(rec.Body).Decode(&errResp)
	if !strings.Contains(errResp.Error.Message, "max_tokens is required") {
		t.Errorf("error message = %q, want 'max_tokens is required'", errResp.Error.Message)
	}
}

// ---------------------------------------------------------------------------
// ServeHTTP — non-streaming success
// ---------------------------------------------------------------------------

func TestServeHTTP_NonStreamSuccess(t *testing.T) {
	oaiResp := adapter.OpenAIResponse{
		ID:    "chatcmpl-test",
		Model: "gpt-4",
		Choices: []adapter.OpenAIChoice{
			{
				Index:   0,
				Message: adapter.OpenAIMessage{Role: "assistant", Content: "Hello!"},
			},
		},
		Usage: &adapter.OpenAIUsage{PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8},
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request was properly transformed
		var oaiReq adapter.OpenAIRequest
		if err := json.NewDecoder(r.Body).Decode(&oaiReq); err != nil {
			t.Errorf("failed to decode upstream request: %v", err)
		}
		if oaiReq.Stream {
			t.Error("non-streaming request should have Stream=false")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(oaiResp)
	}))
	defer upstream.Close()

	cfg := testCfg()
	cfg.UpstreamBaseURL = upstream.URL
	cfg.RequestTimeout = 5e9 // 5s

	c := client.New(cfg, testLogger())
	h := NewMessagesHandler(c, cfg, testLogger())

	anthropicReq := map[string]any{
		"model":      "claude-3",
		"max_tokens": 100,
		"messages":   []any{map[string]any{"role": "user", "content": "hi"}},
	}
	body, _ := json.Marshal(anthropicReq)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var anthropicResp adapter.AnthropicResponse
	if err := json.NewDecoder(rec.Body).Decode(&anthropicResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if anthropicResp.Type != "message" {
		t.Errorf("Type = %q, want message", anthropicResp.Type)
	}
	if anthropicResp.Role != "assistant" {
		t.Errorf("Role = %q, want assistant", anthropicResp.Role)
	}
	if len(anthropicResp.Content) != 1 || anthropicResp.Content[0].Text != "Hello!" {
		t.Errorf("Content = %+v, want [{text: Hello!}]", anthropicResp.Content)
	}
	if anthropicResp.Usage.InputTokens != 5 || anthropicResp.Usage.OutputTokens != 3 {
		t.Errorf("Usage = %+v, want 5/3", anthropicResp.Usage)
	}
}

// ---------------------------------------------------------------------------
// ServeHTTP — upstream error (non-streaming)
// ---------------------------------------------------------------------------

func TestServeHTTP_UpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer upstream.Close()

	cfg := testCfg()
	cfg.UpstreamBaseURL = upstream.URL
	cfg.RequestTimeout = 5e9

	c := client.New(cfg, testLogger())
	h := NewMessagesHandler(c, cfg, testLogger())

	anthropicReq := map[string]any{
		"model":      "claude-3",
		"max_tokens": 100,
		"messages":   []any{map[string]any{"role": "user", "content": "hi"}},
	}
	body, _ := json.Marshal(anthropicReq)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}

	var errResp adapter.AnthropicError
	json.NewDecoder(rec.Body).Decode(&errResp)
	if errResp.Error.Type != "api_error" {
		t.Errorf("error type = %q, want api_error", errResp.Error.Type)
	}
}

// ---------------------------------------------------------------------------
// ServeHTTP — streaming success
// ---------------------------------------------------------------------------

type flushRecorder struct {
	*httptest.ResponseRecorder
	flushed int
}

func (f *flushRecorder) Flush() {
	f.flushed++
	f.ResponseRecorder.Flush()
}

func TestServeHTTP_StreamSuccess(t *testing.T) {
	// Simulate an OpenAI SSE stream
	sseResponse := strings.Join([]string{
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`,
		`data: [DONE]`,
		"",
	}, "\n")

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var oaiReq adapter.OpenAIRequest
		json.NewDecoder(r.Body).Decode(&oaiReq)
		if !oaiReq.Stream {
			t.Error("streaming request should have Stream=true")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(sseResponse))
	}))
	defer upstream.Close()

	cfg := testCfg()
	cfg.UpstreamBaseURL = upstream.URL
	cfg.RequestTimeout = 5e9

	c := client.New(cfg, testLogger())
	h := NewMessagesHandler(c, cfg, testLogger())

	anthropicReq := map[string]any{
		"model":      "claude-3",
		"max_tokens": 100,
		"stream":     true,
		"messages":   []any{map[string]any{"role": "user", "content": "hi"}},
	}
	body, _ := json.Marshal(anthropicReq)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	rec := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	responseBody := rec.Body.String()

	// Should contain expected Anthropic SSE events
	expectedEvents := []string{
		"event: message_start",
		"event: content_block_start",
		"event: content_block_delta",
		"event: content_block_stop",
		"event: message_delta",
		"event: message_stop",
	}

	for _, evt := range expectedEvents {
		if !strings.Contains(responseBody, evt) {
			t.Errorf("response missing event %q", evt)
		}
	}

	// Should have flushed at least once
	if rec.flushed == 0 {
		t.Error("expected Flush to be called at least once")
	}
}

// ---------------------------------------------------------------------------
// ServeHTTP — streaming with tool calls
// ---------------------------------------------------------------------------

func TestServeHTTP_StreamWithToolCalls(t *testing.T) {
	sseResponse := strings.Join([]string{
		`data: {"id":"chatcmpl-2","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-2","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-2","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"loc"}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-2","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ation\":\"NYC\"}"}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-2","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		`data: [DONE]`,
		"",
	}, "\n")

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(sseResponse))
	}))
	defer upstream.Close()

	cfg := testCfg()
	cfg.UpstreamBaseURL = upstream.URL
	cfg.RequestTimeout = 5e9

	c := client.New(cfg, testLogger())
	h := NewMessagesHandler(c, cfg, testLogger())

	anthropicReq := map[string]any{
		"model":      "claude-3",
		"max_tokens": 100,
		"stream":     true,
		"messages":   []any{map[string]any{"role": "user", "content": "weather?"}},
	}
	body, _ := json.Marshal(anthropicReq)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	rec := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}

	h.ServeHTTP(rec, req)

	responseBody := rec.Body.String()

	// Should contain tool_use content block
	if !strings.Contains(responseBody, "tool_use") {
		t.Error("response missing tool_use content block")
	}
	if !strings.Contains(responseBody, "get_weather") {
		t.Error("response missing get_weather tool name")
	}
	if !strings.Contains(responseBody, "input_json_delta") {
		t.Error("response missing input_json_delta")
	}

	// Check message_delta has tool_use stop reason
	if !strings.Contains(responseBody, "message_delta") {
		t.Error("response missing message_delta")
	}
}

// ---------------------------------------------------------------------------
// ServeHTTP — auth header passthrough
// ---------------------------------------------------------------------------

func TestServeHTTP_AuthHeaderPassthrough(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer sk-custom" {
			t.Errorf("upstream Authorization = %q, want 'Bearer sk-custom'", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(adapter.OpenAIResponse{
			Model:   "gpt-4",
			Choices: []adapter.OpenAIChoice{{Message: adapter.OpenAIMessage{Content: "ok"}}},
		})
	}))
	defer upstream.Close()

	cfg := testCfg()
	cfg.UpstreamBaseURL = upstream.URL
	cfg.RequestTimeout = 5e9

	c := client.New(cfg, testLogger())
	h := NewMessagesHandler(c, cfg, testLogger())

	anthropicReq := map[string]any{
		"model":      "claude-3",
		"max_tokens": 100,
		"messages":   []any{map[string]any{"role": "user", "content": "hi"}},
	}
	body, _ := json.Marshal(anthropicReq)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer sk-custom")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// ---------------------------------------------------------------------------
// NewMessagesHandler
// ---------------------------------------------------------------------------

func TestNewMessagesHandler(t *testing.T) {
	cfg := testCfg()
	cfg.UpstreamBaseURL = "http://localhost"
	cfg.RequestTimeout = 5e9

	c := client.New(cfg, testLogger())
	h := NewMessagesHandler(c, cfg, testLogger())

	if h == nil {
		t.Fatal("NewMessagesHandler returned nil")
	}
}

// ---------------------------------------------------------------------------
// writeError — format check
// ---------------------------------------------------------------------------

func TestWriteError(t *testing.T) {
	cfg := testCfg()
	cfg.UpstreamBaseURL = "http://localhost"
	cfg.RequestTimeout = 5e9

	c := client.New(cfg, testLogger())
	h := NewMessagesHandler(c, cfg, testLogger())

	rec := httptest.NewRecorder()
	h.writeError(rec, http.StatusNotFound, "not_found_error", "resource not found")

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	var errResp adapter.AnthropicError
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error: %v", err)
	}
	if errResp.Type != "error" {
		t.Errorf("Type = %q, want error", errResp.Type)
	}
	if errResp.Error.Type != "not_found_error" {
		t.Errorf("Error.Type = %q, want not_found_error", errResp.Error.Type)
	}
	if errResp.Error.Message != "resource not found" {
		t.Errorf("Error.Message = %q, want 'resource not found'", errResp.Error.Message)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
