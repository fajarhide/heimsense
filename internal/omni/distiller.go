package omni

import (
	"context"
	"encoding/json"
	"log/slog"
)

// DistillStats contains metrics about a distillation operation.
type DistillStats struct {
	OriginalTokens  int      `json:"original_tokens"`
	DistilledTokens int      `json:"distilled_tokens"`
	SavedPercent    float64  `json:"saved_percent"`
	FiltersApplied  []string `json:"filters_applied"`
}

// Distiller processes Anthropic messages and distills tool_result content
// through the Omni MCP server to reduce token usage.
type Distiller struct {
	client          *Client
	minContentBytes int
	logger          *slog.Logger
}

// NewDistiller creates a new Distiller.
// minContentBytes is the minimum size of tool_result content (in bytes)
// before distillation is attempted. Content smaller than this is passed through.
func NewDistiller(client *Client, minContentBytes int, logger *slog.Logger) *Distiller {
	if minContentBytes <= 0 {
		minContentBytes = 1024
	}
	return &Distiller{
		client:          client,
		minContentBytes: minContentBytes,
		logger:          logger,
	}
}

// DistillMessages scans the Anthropic message array for tool_result content blocks
// that exceed minContentBytes. Each eligible block is sent to the Omni server
// for distillation. The messages slice is modified in-place.
//
// Returns aggregate stats. If the Omni server is unreachable or returns an error
// for a particular block, that block is left unchanged (fail-open).
func (d *Distiller) DistillMessages(ctx context.Context, messages []map[string]any) (*DistillStats, error) {
	aggregate := &DistillStats{}
	distilled := 0

	for i, msg := range messages {
		role, _ := msg["role"].(string)
		if role != "user" {
			continue
		}

		content, ok := msg["content"]
		if !ok {
			continue
		}

		// Content can be a string or an array of content blocks
		switch c := content.(type) {
		case []any:
			for j, item := range c {
				block, ok := item.(map[string]any)
				if !ok {
					continue
				}
				blockType, _ := block["type"].(string)
				if blockType != "tool_result" {
					continue
				}

				// Extract content from tool_result
				toolContent := extractToolResultContent(block)
				if len(toolContent) < d.minContentBytes {
					continue
				}

				// Send to Omni for distillation
				distilledContent, stats, err := d.client.Distill(ctx, toolContent)
				if err != nil {
					d.logger.Warn("omni distillation failed for tool_result, using original",
						"error", err,
						"content_bytes", len(toolContent),
					)
					continue
				}

				// Update the content block in-place
				block["content"] = distilledContent
				c[j] = block
				distilled++

				// Aggregate stats
				if stats != nil {
					aggregate.OriginalTokens += stats.OriginalTokens
					aggregate.DistilledTokens += stats.DistilledTokens
					for _, f := range stats.FiltersApplied {
						aggregate.FiltersApplied = appendUnique(aggregate.FiltersApplied, f)
					}
				}
			}
			messages[i]["content"] = c
		}
	}

	if aggregate.OriginalTokens > 0 {
		aggregate.SavedPercent = float64(aggregate.OriginalTokens-aggregate.DistilledTokens) / float64(aggregate.OriginalTokens) * 100
	}

	if distilled > 0 {
		d.logger.Info("omni distillation complete",
			"tool_results_distilled", distilled,
			"original_tokens", aggregate.OriginalTokens,
			"distilled_tokens", aggregate.DistilledTokens,
			"saved_percent", aggregate.SavedPercent,
		)
	}

	return aggregate, nil
}

// extractToolResultContent extracts the text content from a tool_result block.
// The content field can be a string or an array of content blocks.
func extractToolResultContent(block map[string]any) string {
	content, ok := block["content"]
	if !ok {
		return ""
	}

	switch c := content.(type) {
	case string:
		return c
	case []any:
		var result string
		for _, item := range c {
			if obj, ok := item.(map[string]any); ok {
				if t, ok := obj["type"].(string); ok && t == "text" {
					if text, ok := obj["text"].(string); ok {
						result += text
					}
				}
			}
		}
		return result
	default:
		// Try JSON serialization as last resort
		bytes, err := json.Marshal(content)
		if err != nil {
			return ""
		}
		return string(bytes)
	}
}

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}
