package client

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

// Client is an HTTP client for the upstream OpenAI-compatible API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	maxRetries int
	logger     *slog.Logger
}

// New creates a new upstream OpenAI client.
func New(cfg *config.Config, logger *slog.Logger) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: cfg.RequestTimeout,
		},
		baseURL:    cfg.UpstreamBaseURL,
		apiKey:     cfg.APIKey,
		maxRetries: cfg.MaxRetries,
		logger:     logger,
	}
}

// ChatCompletion sends a non-streaming chat completion request to the upstream API.
func (c *Client) ChatCompletion(ctx context.Context, req *adapter.OpenAIRequest, authHeader string) (*adapter.OpenAIResponse, error) {
	req.Stream = false

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	respBody, err := c.doWithRetry(ctx, body, authHeader)
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
		c.logger.Error("failed to parse upstream response",
			"error", err,
			"body", string(raw),
		)
		return nil, fmt.Errorf("unmarshal response: %w (body: %s)", err, truncate(string(raw), 200))
	}

	return &oaiResp, nil
}

// ChatCompletionStream sends a streaming chat completion request and returns
// the raw response body for SSE processing. The caller must close the body.
func (c *Client) ChatCompletionStream(ctx context.Context, req *adapter.OpenAIRequest, authHeader string) (io.ReadCloser, error) {
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	respBody, err := c.doWithRetry(ctx, body, authHeader)
	if err != nil {
		return nil, err
	}

	return respBody, nil
}

// doWithRetry performs the HTTP request with exponential backoff retry for transient errors.
func (c *Client) doWithRetry(ctx context.Context, body []byte, authHeader string) (io.ReadCloser, error) {
	url := c.baseURL + "/chat/completions"

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * 500 * time.Millisecond
			c.logger.Warn("retrying upstream request",
				"attempt", attempt,
				"backoff", backoff,
			)
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
		} else if c.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("upstream request: %w", err)
			continue
		}

		// Retry on 5xx errors.
		if resp.StatusCode >= 500 {
			respBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("upstream %d: %s", resp.StatusCode, truncate(string(respBytes), 200))
			c.logger.Warn("upstream server error",
				"status", resp.StatusCode,
				"body", truncate(string(respBytes), 200),
			)
			continue
		}

		// For non-2xx client errors, return the error without retrying.
		if resp.StatusCode >= 400 {
			respBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, truncate(string(respBytes), 500))
		}

		return resp.Body, nil
	}

	return nil, fmt.Errorf("all %d retries exhausted: %w", c.maxRetries, lastErr)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
