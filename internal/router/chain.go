package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/fajarhide/heimsense/internal/adapter"
	"github.com/fajarhide/heimsense/internal/config"
)

// ErrAllProvidersFailed is returned when every provider in the chain has failed.
var ErrAllProvidersFailed = fmt.Errorf("all providers in the chain have failed")

// UpstreamError wraps an error with the HTTP status code from the upstream provider.
type UpstreamError struct {
	StatusCode int
	Message    string
}

func (e *UpstreamError) Error() string {
	return fmt.Sprintf("upstream %d: %s", e.StatusCode, e.Message)
}

// ProviderChain holds an ordered list of providers and attempts them in sequence,
// falling back to the next provider on retryable errors.
type ProviderChain struct {
	links  []providerLink
	logger *slog.Logger
}

type providerLink struct {
	name       string
	baseURL    string
	apiKey     string
	model      string
	maxRetries int
	httpClient *http.Client
}

// NewProviderChain builds a chain from config providers.
// If no providers are configured, it returns nil (caller should use legacy single client).
func NewProviderChain(providers []config.ProviderConfig, requestTimeout time.Duration, logger *slog.Logger) *ProviderChain {
	if len(providers) == 0 {
		return nil
	}

	chain := &ProviderChain{
		logger: logger,
	}

	for _, p := range providers {
		retries := p.MaxRetries
		if retries <= 0 {
			retries = 3
		}
		chain.links = append(chain.links, providerLink{
			name:       p.Name,
			baseURL:    p.BaseURL,
			apiKey:     p.APIKey,
			model:      p.DefaultModel,
			maxRetries: retries,
			httpClient: &http.Client{
				Timeout: requestTimeout,
			},
		})
	}

	return chain
}

// ChatCompletion tries each provider in sequence for a non-streaming request.
// Falls back to the next provider on retryable errors (429, 5xx, timeout).
func (c *ProviderChain) ChatCompletion(ctx context.Context, req *adapter.OpenAIRequest, authHeader string) (*adapter.OpenAIResponse, error) {
	var lastErr error

	for _, link := range c.links {
		c.logger.Info("trying provider", "provider", link.name, "url", link.baseURL)

		resp, err := link.chatCompletion(ctx, req, authHeader)
		if err == nil {
			return resp, nil
		}

		if !shouldFallback(err) {
			// Non-retryable error (4xx client error) — stop immediately
			c.logger.Error("provider returned non-retryable error",
				"provider", link.name, "error", err)
			return nil, err
		}

		c.logger.Warn("provider failed, falling back",
			"provider", link.name, "error", err)
		lastErr = err
	}

	if lastErr != nil {
		return nil, fmt.Errorf("%w: %v", ErrAllProvidersFailed, lastErr)
	}
	return nil, ErrAllProvidersFailed
}

// ChatCompletionStream tries each provider in sequence for a streaming request.
func (c *ProviderChain) ChatCompletionStream(ctx context.Context, req *adapter.OpenAIRequest, authHeader string) (io.ReadCloser, error) {
	var lastErr error

	for _, link := range c.links {
		c.logger.Info("trying provider (stream)", "provider", link.name, "url", link.baseURL)

		body, err := link.chatCompletionStream(ctx, req, authHeader)
		if err == nil {
			return body, nil
		}

		if !shouldFallback(err) {
			c.logger.Error("provider returned non-retryable error (stream)",
				"provider", link.name, "error", err)
			return nil, err
		}

		c.logger.Warn("provider failed (stream), falling back",
			"provider", link.name, "error", err)
		lastErr = err
	}

	if lastErr != nil {
		return nil, fmt.Errorf("%w: %v", ErrAllProvidersFailed, lastErr)
	}
	return nil, ErrAllProvidersFailed
}

// shouldFallback returns true for errors that warrant trying the next provider:
// HTTP 429 (rate limit), 5xx (server errors), and network/timeout errors.
// Returns false for 4xx client errors (bad request, auth error, etc.).
func shouldFallback(err error) bool {
	if ue, ok := err.(*UpstreamError); ok {
		return ue.StatusCode == 429 || ue.StatusCode >= 500
	}
	// Network errors, timeouts, etc. are retryable
	return true
}

// --- per-link execution with retry ---

func (l *providerLink) chatCompletion(ctx context.Context, req *adapter.OpenAIRequest, authHeader string) (*adapter.OpenAIResponse, error) {
	req.Stream = false

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	respBody, err := l.doWithRetry(ctx, body, authHeader)
	if err != nil {
		return nil, err
	}
	defer respBody.Close()

	raw, err := io.ReadAll(respBody)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var oaiResp adapter.OpenAIResponse
	if err := json.Unmarshal(raw, &oaiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w (body: %s)", err, truncStr(string(raw), 200))
	}

	return &oaiResp, nil
}

func (l *providerLink) chatCompletionStream(ctx context.Context, req *adapter.OpenAIRequest, authHeader string) (io.ReadCloser, error) {
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	return l.doWithRetry(ctx, body, authHeader)
}

func (l *providerLink) doWithRetry(ctx context.Context, body []byte, authHeader string) (io.ReadCloser, error) {
	url := l.baseURL + "/chat/completions"

	var lastErr error
	for attempt := 0; attempt <= l.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * 500 * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		} else if l.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+l.apiKey)
		}

		resp, err := l.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("upstream request: %w", err)
			continue
		}

		// Retryable server errors (5xx)
		if resp.StatusCode >= 500 {
			respBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = &UpstreamError{StatusCode: resp.StatusCode, Message: truncStr(string(respBytes), 200)}
			continue
		}

		// Rate limited — retryable but also signals chain to try next provider
		if resp.StatusCode == 429 {
			respBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, &UpstreamError{StatusCode: 429, Message: truncStr(string(respBytes), 200)}
		}

		// Non-retryable client errors (400, 401, 403, etc.)
		if resp.StatusCode >= 400 {
			respBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, &UpstreamError{StatusCode: resp.StatusCode, Message: truncStr(string(respBytes), 500)}
		}

		return resp.Body, nil
	}

	return nil, fmt.Errorf("all %d retries exhausted: %w", l.maxRetries, lastErr)
}

func truncStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
