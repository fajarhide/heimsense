package omni

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Client communicates with the local Omni MCP server for token distillation.
type Client struct {
	httpClient *http.Client
	baseURL    string
	logger     *slog.Logger
}

// NewClient creates a new Omni MCP client.
// The baseURL should be the root URL of the Omni server (e.g. http://localhost:7070).
func NewClient(baseURL string, logger *slog.Logger) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		baseURL: baseURL,
		logger:  logger,
	}
}

// DistillRequest is the payload sent to the Omni distillation endpoint.
type DistillRequest struct {
	Content string   `json:"content"`
	Filters []string `json:"filters,omitempty"`
}

// DistillResponse is the response from the Omni distillation endpoint.
type DistillResponse struct {
	Distilled       string   `json:"distilled"`
	OriginalTokens  int      `json:"original_tokens"`
	DistilledTokens int      `json:"distilled_tokens"`
	FiltersApplied  []string `json:"filters_applied"`
}

// Distill sends content to the Omni MCP server for distillation.
// Returns the distilled content and stats. On any error it returns
// the original content unchanged so the caller can proceed gracefully.
func (c *Client) Distill(ctx context.Context, content string) (string, *DistillStats, error) {
	reqBody := DistillRequest{Content: content}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return content, nil, fmt.Errorf("omni marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/distill", bytes.NewReader(bodyBytes))
	if err != nil {
		return content, nil, fmt.Errorf("omni request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return content, nil, fmt.Errorf("omni call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return content, nil, fmt.Errorf("omni returned %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var distResp DistillResponse
	if err := json.NewDecoder(resp.Body).Decode(&distResp); err != nil {
		return content, nil, fmt.Errorf("omni decode: %w", err)
	}

	savedPct := 0.0
	if distResp.OriginalTokens > 0 {
		savedPct = float64(distResp.OriginalTokens-distResp.DistilledTokens) / float64(distResp.OriginalTokens) * 100
	}

	stats := &DistillStats{
		OriginalTokens:  distResp.OriginalTokens,
		DistilledTokens: distResp.DistilledTokens,
		SavedPercent:    savedPct,
		FiltersApplied:  distResp.FiltersApplied,
	}

	return distResp.Distilled, stats, nil
}

// IsHealthy checks if the Omni MCP server is reachable.
func (c *Client) IsHealthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
