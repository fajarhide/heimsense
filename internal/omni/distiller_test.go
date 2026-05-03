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

func TestDistiller_DistillMessages(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := DistillResponse{
			Distilled:       "distilled result",
			OriginalTokens:  1000,
			DistilledTokens: 100,
			FiltersApplied:  []string{"test-filter"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, logger)
	distiller := NewDistiller(client, 10, logger)

	// Create raw maps similar to what JSON decoding produces
	messages := []map[string]any{
		{
			"role": "user",
			"content": []any{
				map[string]any{
					"type": "text",
					"text": "hello",
				},
				map[string]any{
					"type":        "tool_result",
					"tool_use_id": "tool_123",
					"content":     "this is a very long string that should be distilled",
				},
				map[string]any{
					"type":        "tool_result",
					"tool_use_id": "tool_456",
					"content":     "short", // too short, shouldn't be distilled
				},
			},
		},
	}

	stats, err := distiller.DistillMessages(context.Background(), messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats == nil {
		t.Fatal("expected stats, got nil")
	}

	if stats.OriginalTokens != 1000 || stats.DistilledTokens != 100 {
		t.Errorf("unexpected tokens: %d -> %d", stats.OriginalTokens, stats.DistilledTokens)
	}

	// Check if the message was actually modified
	content := messages[0]["content"].([]any)

	textBlock := content[0].(map[string]any)
	if textBlock["text"] != "hello" {
		t.Errorf("text block modified unexpectedly: %v", textBlock)
	}

	distilledBlock := content[1].(map[string]any)
	if distilledBlock["content"] != "distilled result" {
		t.Errorf("expected distilled content, got %v", distilledBlock["content"])
	}

	shortBlock := content[2].(map[string]any)
	if shortBlock["content"] != "short" {
		t.Errorf("short block modified unexpectedly: %v", shortBlock["content"])
	}
}

func TestExtractToolResultContent(t *testing.T) {
	tests := []struct {
		name  string
		block map[string]any
		want  string
	}{
		{
			name: "string content",
			block: map[string]any{
				"content": "hello",
			},
			want: "hello",
		},
		{
			name: "array of text blocks",
			block: map[string]any{
				"content": []any{
					map[string]any{"type": "text", "text": "hello "},
					map[string]any{"type": "text", "text": "world"},
				},
			},
			want: "hello world",
		},
		{
			name: "unknown format fallback",
			block: map[string]any{
				"content": map[string]any{"foo": "bar"},
			},
			want: `{"foo":"bar"}`, // JSON marshaling fallback
		},
		{
			name:  "no content",
			block: map[string]any{},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractToolResultContent(tt.block)
			if got != tt.want {
				t.Errorf("extractToolResultContent() = %q, want %q", got, tt.want)
			}
		})
	}
}
