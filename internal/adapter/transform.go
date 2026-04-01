package adapter

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fajarhide/heimsense/internal/config"
)

// --- Anthropic Request Types ---

// AnthropicRequest represents an incoming Anthropic /v1/messages request.
type AnthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []AnthropicMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens"`
	System      any                `json:"system,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
	Temperature *float64           `json:"temperature,omitempty"`
	TopP        *float64           `json:"top_p,omitempty"`
	StopSeq     []string           `json:"stop_sequences,omitempty"`
	Tools       []AnthropicTool    `json:"tools,omitempty"`
	ToolChoice  any                `json:"tool_choice,omitempty"`
}

type AnthropicTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema"`
}

// AnthropicMessage represents a single message in the Anthropic format.
type AnthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string OR []any -> ContentBlocks
}

// ContentBlock represents a typed content block in the Anthropic format.
type ContentBlock struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	ID        string `json:"id,omitempty"`          // tool_use
	Name      string `json:"name,omitempty"`        // tool_use
	Input     any    `json:"input,omitempty"`       // tool_use
	ToolUseID string `json:"tool_use_id,omitempty"` // tool_result
	Content   any    `json:"content,omitempty"`     // tool_result content (string or array)
	IsError   bool   `json:"is_error,omitempty"`    // tool_result
}

// --- Anthropic Response Types ---

// AnthropicResponse represents the Anthropic /v1/messages response.
type AnthropicResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   *string        `json:"stop_reason"`
	StopSequence *string        `json:"stop_sequence"`
	Usage        AnthropicUsage `json:"usage"`
}

// AnthropicUsage reports token usage in the Anthropic response.
type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// AnthropicError is the error envelope returned by the Anthropic API format.
type AnthropicError struct {
	Type  string              `json:"type"`
	Error AnthropicErrorInner `json:"error"`
}

// AnthropicErrorInner is the inner error object.
type AnthropicErrorInner struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// --- OpenAI Request Types ---

// OpenAIRequest represents an outgoing OpenAI /v1/chat/completions request.
type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	Stream      bool            `json:"stream"`
	Stop        []string        `json:"stop,omitempty"`
	Tools       []OpenAITool    `json:"tools,omitempty"`
}

type OpenAITool struct {
	Type     string         `json:"type"`
	Function OpenAIFunction `json:"function"`
}

type OpenAIFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters"`
}

// OpenAIMessage represents a single message in the OpenAI format.
type OpenAIMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content"`
	ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	Name       string           `json:"name,omitempty"`
}

type OpenAIToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function OpenAIToolCallFunction `json:"function"`
}

type OpenAIToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// --- OpenAI Response Types ---

// OpenAIResponse represents the OpenAI /v1/chat/completions response.
type OpenAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   *OpenAIUsage   `json:"usage,omitempty"`
}

// OpenAIChoice is a single completion choice.
type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	Delta        *OpenAIDelta  `json:"delta,omitempty"`
	FinishReason *string       `json:"finish_reason"`
}

// OpenAIDelta is the streaming delta object.
type OpenAIDelta struct {
	Role      string                `json:"role,omitempty"`
	Content   string                `json:"content,omitempty"`
	ToolCalls []OpenAIDeltaToolCall `json:"tool_calls,omitempty"`
}

type OpenAIDeltaToolCall struct {
	Index    int                          `json:"index"`
	ID       string                       `json:"id,omitempty"`
	Type     string                       `json:"type,omitempty"`
	Function *OpenAIDeltaToolCallFunction `json:"function,omitempty"`
}

type OpenAIDeltaToolCallFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// OpenAIUsage reports token usage in the OpenAI response.
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// --- Transformation Functions ---

// normalizeModel maps Anthropic model names to common upstream compatible names
func normalizeModel(model string, forceModel string, cfg *config.Config) (string, error) {
	// FORCE MODEL ALWAYS WINS. NO EXCEPTIONS.
	if forceModel != "" {
		return forceModel, nil
	}

	// Dynamic config mapping
	if cfg != nil {
		switch {
		case strings.Contains(model, "haiku"):
			if cfg.ModelMapHaiku != "" {
				return cfg.ModelMapHaiku, nil
			}
		case strings.Contains(model, "sonnet"):
			if cfg.ModelMapSonnet != "" {
				return cfg.ModelMapSonnet, nil
			}
		case strings.Contains(model, "opus"):
			if cfg.ModelMapOpus != "" {
				return cfg.ModelMapOpus, nil
			}
		}

		// If a matching model alias isn't defined, but the user HAS an Anthropic Custom Model (DefaultModel),
		// we fallback to that custom model so they don't get 400 errors for unrecognized models.
		if (strings.Contains(model, "haiku") || strings.Contains(model, "sonnet") || strings.Contains(model, "opus")) && cfg.DefaultModel != "" {
			return cfg.DefaultModel, nil
		}
	}

	// Default: pass through whatever model was requested
	return model, nil
}

// ToOpenAIRequest converts an Anthropic request into an OpenAI chat completion request.
func ToOpenAIRequest(req *AnthropicRequest, defaultModel, forceModel string, cfg *config.Config) (*OpenAIRequest, error) {
	model, err := normalizeModel(req.Model, forceModel, cfg)
	if err != nil {
		return nil, err
	}

	oaiReq := &OpenAIRequest{
		Model:       model,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
		Stop:        req.StopSeq,
	}

	if forceModel != "" {
		oaiReq.Model, _ = normalizeModel(forceModel, "", cfg)
	} else if oaiReq.Model == "" && defaultModel != "" {
		oaiReq.Model, _ = normalizeModel(defaultModel, "", cfg)
	}

	// Tools mapping
	if len(req.Tools) > 0 {
		oaiReq.Tools = make([]OpenAITool, len(req.Tools))
		for i, t := range req.Tools {
			oaiReq.Tools[i] = OpenAITool{
				Type: "function",
				Function: OpenAIFunction{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.InputSchema,
				},
			}
		}
	}

	// Handle system prompt
	if req.System != nil {
		systemText := extractTextContent(req.System)
		if systemText != "" {
			oaiReq.Messages = append(oaiReq.Messages, OpenAIMessage{
				Role:    "system",
				Content: systemText,
			})
		}
	}

	// Convert each message.
	for _, msg := range req.Messages {
		if msg.Role == "user" {
			switch content := msg.Content.(type) {
			case string:
				oaiReq.Messages = append(oaiReq.Messages, OpenAIMessage{
					Role:    "user",
					Content: content,
				})
			case []any:
				var textParts []string
				for _, item := range content {
					if obj, ok := item.(map[string]any); ok {
						t, _ := obj["type"].(string)
						if t == "text" {
							if txt, ok := obj["text"].(string); ok {
								textParts = append(textParts, txt)
							}
						} else if t == "tool_result" {
							// Flush preceding text as an independent user message
							if len(textParts) > 0 {
								oaiReq.Messages = append(oaiReq.Messages, OpenAIMessage{
									Role:    "user",
									Content: strings.Join(textParts, "\n"),
								})
								textParts = nil
							}
							toolID, _ := obj["tool_use_id"].(string)
							var tc string
							if c, ok := obj["content"]; ok {
								tc = extractTextContent(c)
							}
							oaiReq.Messages = append(oaiReq.Messages, OpenAIMessage{
								Role:       "tool",
								ToolCallID: toolID,
								Content:    tc,
							})
						}
					}
				}
				// Flush remaining
				if len(textParts) > 0 {
					oaiReq.Messages = append(oaiReq.Messages, OpenAIMessage{
						Role:    "user",
						Content: strings.Join(textParts, "\n"),
					})
				}
			}
		} else if msg.Role == "assistant" {
			switch content := msg.Content.(type) {
			case string:
				oaiReq.Messages = append(oaiReq.Messages, OpenAIMessage{
					Role:    "assistant",
					Content: content,
				})
			case []any:
				var textParts []string
				var toolCalls []OpenAIToolCall
				for _, item := range content {
					if obj, ok := item.(map[string]any); ok {
						t, _ := obj["type"].(string)
						if t == "text" {
							if txt, ok := obj["text"].(string); ok {
								textParts = append(textParts, txt)
							}
						} else if t == "tool_use" {
							id, _ := obj["id"].(string)
							name, _ := obj["name"].(string)
							inputBytes, _ := json.Marshal(obj["input"])

							toolCalls = append(toolCalls, OpenAIToolCall{
								ID:   id,
								Type: "function",
								Function: OpenAIToolCallFunction{
									Name:      name,
									Arguments: string(inputBytes),
								},
							})
						}
					}
				}
				oaiReq.Messages = append(oaiReq.Messages, OpenAIMessage{
					Role:      "assistant",
					Content:   strings.Join(textParts, "\n"),
					ToolCalls: toolCalls,
				})
			}
		}
	}

	return oaiReq, nil
}

// ToAnthropicResponse converts an OpenAI response into an Anthropic response.
func ToAnthropicResponse(oaiResp *OpenAIResponse) *AnthropicResponse {
	resp := &AnthropicResponse{
		ID:   generateMessageID(),
		Type: "message",
		Role: "assistant",
	}

	if oaiResp.Model != "" {
		resp.Model = oaiResp.Model
	}

	// Extract content from the first choice.
	if len(oaiResp.Choices) > 0 {
		choice := oaiResp.Choices[0]
		resp.Content = []ContentBlock{}

		if choice.Message.Content != "" {
			resp.Content = append(resp.Content, ContentBlock{
				Type: "text",
				Text: choice.Message.Content,
			})
		}

		for _, tc := range choice.Message.ToolCalls {
			var input map[string]any
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
			if input == nil {
				input = make(map[string]any)
			}
			resp.Content = append(resp.Content, ContentBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: input,
			})
		}

		if choice.FinishReason != nil {
			reason := mapFinishReason(*choice.FinishReason)
			resp.StopReason = &reason
		}
	} else {
		resp.Content = []ContentBlock{}
	}

	// Map usage.
	if oaiResp.Usage != nil {
		resp.Usage = AnthropicUsage{
			InputTokens:  oaiResp.Usage.PromptTokens,
			OutputTokens: oaiResp.Usage.CompletionTokens,
		}
	}

	return resp
}

// NewAnthropicError creates an Anthropic-formatted error response.
func NewAnthropicError(errType, errMessage string) *AnthropicError {
	return &AnthropicError{
		Type: "error",
		Error: AnthropicErrorInner{
			Type:    errType,
			Message: errMessage,
		},
	}
}

// --- Streaming Event Types ---

// StreamEvent represents a single SSE event in the Anthropic streaming format.
type StreamEvent struct {
	Event string `json:"-"`
	Data  any    `json:"data,omitempty"`
}

// MessageStartEvent is sent at the start of a streaming response.
type MessageStartEvent struct {
	Type    string             `json:"type"`
	Message *AnthropicResponse `json:"message"`
}

// ContentBlockStartEvent signals the beginning of a content block.
type ContentBlockStartEvent struct {
	Type         string       `json:"type"`
	Index        int          `json:"index"`
	ContentBlock ContentBlock `json:"content_block"`
}

// ContentBlockDeltaEvent carries incremental text content.
type ContentBlockDeltaEvent struct {
	Type  string     `json:"type"`
	Index int        `json:"index"`
	Delta DeltaBlock `json:"delta"`
}

// DeltaBlock carries the actual text or tool JSON delta.
type DeltaBlock struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
}

// ContentBlockStopEvent signals the end of a content block.
type ContentBlockStopEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
}

// MessageDeltaEvent carries the stop reason at the end.
type MessageDeltaEvent struct {
	Type  string          `json:"type"`
	Delta MessageDelta    `json:"delta"`
	Usage *AnthropicUsage `json:"usage,omitempty"`
}

// MessageDelta is the delta payload in a message_delta event.
type MessageDelta struct {
	StopReason   *string `json:"stop_reason"`
	StopSequence *string `json:"stop_sequence"`
}

// MessageStopEvent signals the end of the message stream.
type MessageStopEvent struct {
	Type string `json:"type"`
}

// --- Helpers ---

// extractTextContent extracts a plain text string from Anthropic's polymorphic content.
func extractTextContent(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var result string
		for _, item := range v {
			if obj, ok := item.(map[string]any); ok {
				if t, ok := obj["text"].(string); ok {
					result += t
				}
			}
		}
		return result
	case []ContentBlock:
		var result string
		for _, block := range v {
			if block.Type == "text" {
				result += block.Text
			}
		}
		return result
	default:
		return fmt.Sprintf("%v", content)
	}
}

// mapFinishReason converts an OpenAI finish_reason to an Anthropic stop_reason.
func mapFinishReason(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "content_filter":
		return "end_turn"
	case "tool_calls":
		return "tool_use"
	default:
		return "end_turn"
	}
}

// generateMessageID produces a unique Anthropic-style message ID.
func generateMessageID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return "msg_" + hex.EncodeToString(b)
}

// BuildMessageStartEvent creates the message_start event.
func BuildMessageStartEvent(model string) (*AnthropicResponse, StreamEvent) {
	msg := &AnthropicResponse{
		ID:      generateMessageID(),
		Type:    "message",
		Role:    "assistant",
		Model:   model,
		Content: []ContentBlock{},
		Usage:   AnthropicUsage{},
	}

	return msg, StreamEvent{
		Event: "message_start",
		Data: MessageStartEvent{
			Type:    "message_start",
			Message: msg,
		},
	}
}

// BuildStreamStopEvents creates the final SSE events to close a stream.
func BuildStreamStopEvents(usage *OpenAIUsage, finishReason *string) []StreamEvent {
	var stopReason string
	if finishReason != nil {
		stopReason = mapFinishReason(*finishReason)
	} else {
		stopReason = "end_turn"
	}

	var anthropicUsage *AnthropicUsage
	if usage != nil {
		anthropicUsage = &AnthropicUsage{
			InputTokens:  usage.PromptTokens,
			OutputTokens: usage.CompletionTokens,
		}
	}

	return []StreamEvent{
		{Event: "message_delta", Data: MessageDeltaEvent{
			Type: "message_delta",
			Delta: MessageDelta{
				StopReason:   &stopReason,
				StopSequence: nil,
			},
			Usage: anthropicUsage,
		}},
		{Event: "message_stop", Data: MessageStopEvent{Type: "message_stop"}},
	}
}
