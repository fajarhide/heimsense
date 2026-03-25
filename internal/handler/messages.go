package handler

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/fajarhide/heimsense/internal/adapter"
	"github.com/fajarhide/heimsense/internal/client"
	"github.com/fajarhide/heimsense/internal/config"
)

// MessagesHandler handles Anthropic /v1/messages requests.
type MessagesHandler struct {
	client *client.Client
	cfg    *config.Config
	logger *slog.Logger
}

// NewMessagesHandler creates a new handler for the /v1/messages endpoint.
func NewMessagesHandler(c *client.Client, cfg *config.Config, logger *slog.Logger) *MessagesHandler {
	return &MessagesHandler{
		client: c,
		cfg:    cfg,
		logger: logger,
	}
}

// ServeHTTP implements http.Handler.
func (h *MessagesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method not allowed")
		return
	}

	start := time.Now()

	// Parse request body.
	var req adapter.AnthropicRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("failed to decode request", "error", err)
		h.writeError(w, http.StatusBadRequest, "invalid_request_error", "invalid JSON: "+err.Error())
		return
	}

	// Validate required fields.
	if len(req.Messages) == 0 {
		h.writeError(w, http.StatusBadRequest, "invalid_request_error", "messages is required")
		return
	}
	if req.MaxTokens == 0 {
		h.writeError(w, http.StatusBadRequest, "invalid_request_error", "max_tokens is required")
		return
	}

	authHeader := r.Header.Get("Authorization")

	h.logger.Info("incoming request",
		"model", req.Model,
		"stream", req.Stream,
		"messages", len(req.Messages),
		"max_tokens", req.MaxTokens,
	)

	// Transform to OpenAI format.
	oaiReq := adapter.ToOpenAIRequest(&req, h.cfg.DefaultModel, h.cfg.ForceModel)

	if req.Stream {
		h.handleStream(w, r, oaiReq, authHeader, start)
	} else {
		h.handleNonStream(w, r, oaiReq, authHeader, start)
	}
}

// handleNonStream processes a non-streaming request.
func (h *MessagesHandler) handleNonStream(w http.ResponseWriter, r *http.Request, oaiReq *adapter.OpenAIRequest, authHeader string, start time.Time) {
	oaiResp, err := h.client.ChatCompletion(r.Context(), oaiReq, authHeader)
	if err != nil {
		h.logger.Error("upstream request failed", "error", err, "duration", time.Since(start))
		h.writeError(w, http.StatusBadGateway, "api_error", "upstream error: "+err.Error())
		return
	}

	anthropicResp := adapter.ToAnthropicResponse(oaiResp)

	h.logger.Info("request completed",
		"duration", time.Since(start),
		"model", anthropicResp.Model,
		"input_tokens", anthropicResp.Usage.InputTokens,
		"output_tokens", anthropicResp.Usage.OutputTokens,
	)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(anthropicResp); err != nil {
		h.logger.Error("failed to encode response", "error", err)
	}
}

// handleStream processes a streaming request using SSE.
func (h *MessagesHandler) handleStream(w http.ResponseWriter, r *http.Request, oaiReq *adapter.OpenAIRequest, authHeader string, start time.Time) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeError(w, http.StatusInternalServerError, "api_error", "streaming not supported")
		return
	}

	body, err := h.client.ChatCompletionStream(r.Context(), oaiReq, authHeader)
	if err != nil {
		h.logger.Error("upstream stream request failed", "error", err, "duration", time.Since(start))
		h.writeError(w, http.StatusBadGateway, "api_error", "upstream error: "+err.Error())
		return
	}
	defer body.Close()

	// Set SSE headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send initial Anthropic stream events.
	msg, startEvent := adapter.BuildMessageStartEvent(oaiReq.Model)
	_ = msg
	h.writeSSE(w, startEvent)
	flusher.Flush()

	// Read OpenAI SSE stream and translate to Anthropic SSE.
	var lastUsage *adapter.OpenAIUsage
	var finishReason *string
	activeContentIndex := -1
	inTextBlock := false
	inToolBlock := false

	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk adapter.OpenAIResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			h.logger.Warn("failed to parse stream chunk", "error", err, "data", data)
			continue
		}

		if chunk.Usage != nil {
			lastUsage = chunk.Usage
		}

		if len(chunk.Choices) > 0 {
			choice := chunk.Choices[0]
			if choice.FinishReason != nil {
				finishReason = choice.FinishReason
			}

			delta := choice.Delta
			if delta == nil {
				continue
			}

			// Process text content
			if delta.Content != "" {
				if inToolBlock {
					h.writeSSE(w, adapter.StreamEvent{
						Event: "content_block_stop",
						Data:  adapter.ContentBlockStopEvent{Type: "content_block_stop", Index: activeContentIndex},
					})
					inToolBlock = false
				}

				if !inTextBlock {
					activeContentIndex++
					h.writeSSE(w, adapter.StreamEvent{
						Event: "content_block_start",
						Data: adapter.ContentBlockStartEvent{
							Type:         "content_block_start",
							Index:        activeContentIndex,
							ContentBlock: adapter.ContentBlock{Type: "text", Text: ""},
						},
					})
					inTextBlock = true
				}

				h.writeSSE(w, adapter.StreamEvent{
					Event: "content_block_delta",
					Data: adapter.ContentBlockDeltaEvent{
						Type:  "content_block_delta",
						Index: activeContentIndex,
						Delta: adapter.DeltaBlock{Type: "text_delta", Text: delta.Content},
					},
				})
				flusher.Flush()
			}

			// Process tool calls
			for _, tc := range delta.ToolCalls {
				if tc.ID != "" || (tc.Function != nil && tc.Function.Name != "") {
					// A new tool call is starting
					// Close previous block if any
					if inTextBlock || inToolBlock {
						h.writeSSE(w, adapter.StreamEvent{
							Event: "content_block_stop",
							Data:  adapter.ContentBlockStopEvent{Type: "content_block_stop", Index: activeContentIndex},
						})
						inTextBlock = false
						inToolBlock = false
					}

					activeContentIndex++
					var name string
					if tc.Function != nil {
						name = tc.Function.Name
					}

					h.writeSSE(w, adapter.StreamEvent{
						Event: "content_block_start",
						Data: adapter.ContentBlockStartEvent{
							Type:         "content_block_start",
							Index:        activeContentIndex,
							ContentBlock: adapter.ContentBlock{Type: "tool_use", ID: tc.ID, Name: name},
						},
					})
					inToolBlock = true
				}

				if tc.Function != nil && tc.Function.Arguments != "" {
					if !inToolBlock {
						// Resume existing block (if text preceded in the same chunk)
						// Not strictly correct without tracking IDs, but covers most cases
					}
					h.writeSSE(w, adapter.StreamEvent{
						Event: "content_block_delta",
						Data: adapter.ContentBlockDeltaEvent{
							Type:  "content_block_delta",
							Index: activeContentIndex,
							Delta: adapter.DeltaBlock{Type: "input_json_delta", PartialJSON: tc.Function.Arguments},
						},
					})
				}
				flusher.Flush()
			}
		}
	}

	// Close the last active block if needed
	if inTextBlock || inToolBlock {
		h.writeSSE(w, adapter.StreamEvent{
			Event: "content_block_stop",
			Data:  adapter.ContentBlockStopEvent{Type: "content_block_stop", Index: activeContentIndex},
		})
	}

	// Send closing events.
	stopEvents := adapter.BuildStreamStopEvents(lastUsage, finishReason)
	for _, evt := range stopEvents {
		h.writeSSE(w, evt)
	}
	flusher.Flush()

	h.logger.Info("stream completed", "duration", time.Since(start))
}

// writeSSE writes a single SSE event to the response writer.
func (h *MessagesHandler) writeSSE(w http.ResponseWriter, evt adapter.StreamEvent) {
	fmt.Fprintf(w, "event: %s\n", evt.Event)
	if evt.Data != nil {
		data, err := json.Marshal(evt.Data)
		if err != nil {
			h.logger.Error("failed to marshal SSE data", "error", err)
			return
		}
		fmt.Fprintf(w, "data: %s\n", string(data))
	}
	fmt.Fprint(w, "\n")
}

// writeError sends a JSON error response in Anthropic format.
func (h *MessagesHandler) writeError(w http.ResponseWriter, statusCode int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errResp := adapter.NewAnthropicError(errType, message)
	if err := json.NewEncoder(w).Encode(errResp); err != nil {
		h.logger.Error("failed to write error response", "error", err)
	}
}

// HealthHandler returns a simple health check response.
func HealthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"status":"ok"}`)
}
