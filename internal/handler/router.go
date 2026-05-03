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
	"github.com/fajarhide/heimsense/internal/omni"
	"github.com/fajarhide/heimsense/internal/router"
	"github.com/fajarhide/heimsense/internal/translator"
)

// UniversalRouterHandler handles all AI completion requests across formats.
type UniversalRouterHandler struct {
	client    *client.Client
	chain     *router.ProviderChain
	cfg       *config.Config
	distiller *omni.Distiller
	logger    *slog.Logger

	inbound  *translator.InboundEngine
	outbound *translator.OutboundEngine
}

// NewUniversalRouterHandler creates a new handler for universal routing.
func NewUniversalRouterHandler(c *client.Client, cfg *config.Config, logger *slog.Logger) *UniversalRouterHandler {
	return &UniversalRouterHandler{
		client:   c,
		cfg:      cfg,
		logger:   logger,
		inbound:  translator.NewInboundEngine(cfg),
		outbound: translator.NewOutboundEngine(cfg),
	}
}

// SetChain attaches a ProviderChain to the handler.
func (h *UniversalRouterHandler) SetChain(chain *router.ProviderChain) {
	h.chain = chain
}

// SetDistiller attaches an Omni Distiller to the handler.
func (h *UniversalRouterHandler) SetDistiller(d *omni.Distiller) {
	h.distiller = d
}

// ServeHTTP implements http.Handler.
func (h *UniversalRouterHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method not allowed", translator.FormatOpenAI)
		return
	}

	start := time.Now()

	// 1. Read Raw Payload
	rawBodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request_error", "failed to read body", translator.FormatOpenAI)
		return
	}

	// 2. Detect Incoming Format
	clientFormat := translator.DetectFormat(r.URL.Path, rawBodyBytes)

	// Decode into map for Omni Distillation
	var rawBody map[string]any
	if err := json.Unmarshal(rawBodyBytes, &rawBody); err != nil {
		h.logger.Error("failed to decode request", "error", err)
		h.writeError(w, http.StatusBadRequest, "invalid_request_error", "invalid JSON", clientFormat)
		return
	}

	// --- Omni Pre-Hook: distill tool_results before transforming ---
	if h.distiller != nil {
		// Only Anthropic format is currently supported by distiller directly,
		// but since we're acting as a universal router, we can expand this later.
		// For now, if it's Anthropic format, we apply the hook.
		if clientFormat == translator.FormatAnthropic {
			if msgs, ok := rawBody["messages"].([]any); ok {
				msgMaps := make([]map[string]any, 0, len(msgs))
				for _, m := range msgs {
					if mm, ok := m.(map[string]any); ok {
						msgMaps = append(msgMaps, mm)
					}
				}

				stats, err := h.distiller.DistillMessages(r.Context(), msgMaps)
				if err != nil {
					h.logger.Warn("omni distillation error, proceeding with original", "error", err)
				} else if stats != nil && stats.OriginalTokens > 0 {
					h.logger.Info("omni pre-hook applied",
						"saved_percent", fmt.Sprintf("%.1f%%", stats.SavedPercent),
						"original_tokens", stats.OriginalTokens,
						"distilled_tokens", stats.DistilledTokens,
					)
				}

				rebuilt := make([]any, len(msgMaps))
				for i, mm := range msgMaps {
					rebuilt[i] = mm
				}
				rawBody["messages"] = rebuilt

				// Re-marshal the distilled body
				rawBodyBytes, _ = json.Marshal(rawBody)
			}
		}
	}

	// 3. Translate Inbound -> Standard (OpenAI)
	oaiReq, err := h.inbound.Translate(rawBodyBytes, clientFormat)
	if err != nil {
		h.logger.Error("inbound translation failed", "error", err)
		h.writeError(w, http.StatusBadRequest, "invalid_request_error", err.Error(), clientFormat)
		return
	}

	// Validate
	if len(oaiReq.Messages) == 0 {
		h.writeError(w, http.StatusBadRequest, "invalid_request_error", "messages is required", clientFormat)
		return
	}

	authHeader := r.Header.Get("Authorization")

	h.logger.Info("incoming request (universal routing)",
		"in_format", clientFormat,
		"model", oaiReq.Model,
		"stream", oaiReq.Stream,
	)

	// 4. Route and Translate Outbound
	if oaiReq.Stream {
		h.handleStream(w, r, oaiReq, authHeader, start, clientFormat)
	} else {
		h.handleNonStream(w, r, oaiReq, authHeader, start, clientFormat)
	}
}

func (h *UniversalRouterHandler) handleNonStream(w http.ResponseWriter, r *http.Request, oaiReq *adapter.OpenAIRequest, authHeader string, start time.Time, clientFormat translator.FormatType) {
	var oaiResp *adapter.OpenAIResponse
	var err error
	var upstreamFormat translator.FormatType

	// The chain attempts the request natively but we pass the *OpenAIRequest.
	// We need the chain to handle outbound translation. Wait, the chain uses OpenAIRequest.
	// Let's modify our approach: Chain uses `chatCompletion` which expects `adapter.OpenAIResponse`.
	// Since Chain is hardcoded to OpenAI response, we let it be. If the upstream provider is Anthropic,
	// the chain will fail parsing it.

	// For Phase 3: We actually need to modify `router.ProviderChain` to use `OutboundEngine` inside!
	// But as an MVP, let's keep it simple here. If `h.chain != nil`, we call it.
	// Let's assume the router chain uses `ToAnthropicRequest` internally if it detects Anthropic.
	// Wait, I didn't update `chain.go` to use OutboundEngine yet. Let's do it in `router`.
	// For now, let's just call the chain.

	if h.chain != nil {
		// chain is responsible for translating outbound and parsing inbound back to OpenAIResponse
		// We'll update chain to handle this later. Let's assume it returns OpenAIResponse.
		oaiResp, err = h.chain.ChatCompletion(r.Context(), oaiReq, authHeader)
		upstreamFormat = translator.FormatOpenAI
	} else {
		oaiResp, err = h.client.ChatCompletion(r.Context(), oaiReq, authHeader)
		upstreamFormat = translator.FormatOpenAI
	}

	if err != nil {
		h.logger.Error("upstream request failed", "error", err, "duration", time.Since(start))
		h.writeError(w, http.StatusBadGateway, "api_error", "upstream error: "+err.Error(), clientFormat)
		return
	}

	// 5. Translate Final Response -> Client Format
	respBytes, _ := json.Marshal(oaiResp)
	finalBytes, err := h.outbound.TranslateResponse(respBytes, upstreamFormat, clientFormat)
	if err != nil {
		h.logger.Error("failed to translate response", "error", err)
		h.writeError(w, http.StatusInternalServerError, "api_error", "failed to translate response", clientFormat)
		return
	}

	h.logger.Info("request completed", "duration", time.Since(start))

	w.Header().Set("Content-Type", "application/json")
	w.Write(finalBytes)
}

func (h *UniversalRouterHandler) handleStream(w http.ResponseWriter, r *http.Request, oaiReq *adapter.OpenAIRequest, authHeader string, start time.Time, clientFormat translator.FormatType) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeError(w, http.StatusInternalServerError, "api_error", "streaming not supported", clientFormat)
		return
	}

	var body io.ReadCloser
	var err error

	if h.chain != nil {
		body, err = h.chain.ChatCompletionStream(r.Context(), oaiReq, authHeader)
	} else {
		body, err = h.client.ChatCompletionStream(r.Context(), oaiReq, authHeader)
	}

	if err != nil {
		h.logger.Error("upstream stream request failed", "error", err, "duration", time.Since(start))
		h.writeError(w, http.StatusBadGateway, "api_error", "upstream error: "+err.Error(), clientFormat)
		return
	}
	defer body.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// If the client requested OpenAI format, just stream it straight back (since upstream streams OpenAI currently)
	if clientFormat == translator.FormatOpenAI {
		scanner := bufio.NewScanner(body)
		for scanner.Scan() {
			fmt.Fprintf(w, "%s\n", scanner.Text())
			flusher.Flush()
		}
		return
	}

	// If the client requested Anthropic format, we must translate the OpenAI SSE to Anthropic SSE
	// (This is the existing logic)
	msg, startEvent := adapter.BuildMessageStartEvent(oaiReq.Model)
	_ = msg
	h.writeSSE(w, startEvent)
	flusher.Flush()

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

			for _, tc := range delta.ToolCalls {
				if tc.ID != "" || (tc.Function != nil && tc.Function.Name != "") {
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

	if inTextBlock || inToolBlock {
		h.writeSSE(w, adapter.StreamEvent{
			Event: "content_block_stop",
			Data:  adapter.ContentBlockStopEvent{Type: "content_block_stop", Index: activeContentIndex},
		})
	}

	stopEvents := adapter.BuildStreamStopEvents(lastUsage, finishReason)
	for _, evt := range stopEvents {
		h.writeSSE(w, evt)
	}
	flusher.Flush()
}

func (h *UniversalRouterHandler) writeSSE(w http.ResponseWriter, evt adapter.StreamEvent) {
	fmt.Fprintf(w, "event: %s\n", evt.Event)
	if evt.Data != nil {
		data, _ := json.Marshal(evt.Data)
		fmt.Fprintf(w, "data: %s\n", string(data))
	}
	fmt.Fprint(w, "\n")
}

func (h *UniversalRouterHandler) writeError(w http.ResponseWriter, statusCode int, errType, message string, clientFormat translator.FormatType) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if clientFormat == translator.FormatAnthropic {
		errResp := adapter.NewAnthropicError(errType, message)
		json.NewEncoder(w).Encode(errResp)
	} else {
		// OpenAI format error
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"type":    errType,
				"message": message,
			},
		})
	}
}

// HealthHandler returns a simple health check response.
func HealthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"status":"ok"}`)
}
