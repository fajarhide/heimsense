package adapter

import (
	"encoding/json"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// ToOpenAIRequest
// ---------------------------------------------------------------------------

func TestToOpenAIRequest_BasicConversion(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "claude-3",
		MaxTokens: 100,
		Messages: []AnthropicMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	oai, _ := ToOpenAIRequest(req, "", "", nil)

	if oai.Model != "claude-3" {
		t.Errorf("Model = %q, want %q", oai.Model, "claude-3")
	}
	if oai.MaxTokens != 100 {
		t.Errorf("MaxTokens = %d, want 100", oai.MaxTokens)
	}
	if len(oai.Messages) != 1 {
		t.Fatalf("Messages length = %d, want 1", len(oai.Messages))
	}
	if oai.Messages[0].Role != "user" || oai.Messages[0].Content != "Hello" {
		t.Errorf("Messages[0] = %+v, want user/Hello", oai.Messages[0])
	}
}

func TestToOpenAIRequest_DefaultModel(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "",
		MaxTokens: 50,
		Messages:  []AnthropicMessage{{Role: "user", Content: "hi"}},
	}

	oai, _ := ToOpenAIRequest(req, "gpt-4-default", "", nil)
	if oai.Model != "gpt-4-default" {
		t.Errorf("Model = %q, want %q", oai.Model, "gpt-4-default")
	}
}

func TestToOpenAIRequest_ForceModelOverride(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "claude-3",
		MaxTokens: 50,
		Messages:  []AnthropicMessage{{Role: "user", Content: "hi"}},
	}

	oai, _ := ToOpenAIRequest(req, "gpt-4-default", "gpt-4-forced", nil)
	if oai.Model != "gpt-4-forced" {
		t.Errorf("Model = %q, want %q (force should override)", oai.Model, "gpt-4-forced")
	}
}

func TestToOpenAIRequest_ForceModelOverridesDefault(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "",
		MaxTokens: 50,
		Messages:  []AnthropicMessage{{Role: "user", Content: "hi"}},
	}

	oai, _ := ToOpenAIRequest(req, "gpt-4-default", "gpt-4-forced", nil)
	if oai.Model != "gpt-4-forced" {
		t.Errorf("Model = %q, want %q (force takes priority over default)", oai.Model, "gpt-4-forced")
	}
}

func TestToOpenAIRequest_SystemPromptString(t *testing.T) {
	system := "You are a helpful assistant."
	req := &AnthropicRequest{
		Model:     "claude-3",
		MaxTokens: 50,
		System:    system,
		Messages:  []AnthropicMessage{{Role: "user", Content: "hi"}},
	}

	oai, _ := ToOpenAIRequest(req, "", "", nil)
	if len(oai.Messages) != 2 {
		t.Fatalf("Messages length = %d, want 2 (system + user)", len(oai.Messages))
	}
	if oai.Messages[0].Role != "system" {
		t.Errorf("Messages[0].Role = %q, want system", oai.Messages[0].Role)
	}
	if oai.Messages[0].Content != system {
		t.Errorf("Messages[0].Content = %q, want %q", oai.Messages[0].Content, system)
	}
}

func TestToOpenAIRequest_SystemPromptArray(t *testing.T) {
	// Simulate []any with text blocks, as it arrives from JSON decoding
	system := []any{
		map[string]any{"type": "text", "text": "System part 1"},
		map[string]any{"type": "text", "text": "System part 2"},
	}
	req := &AnthropicRequest{
		Model:     "claude-3",
		MaxTokens: 50,
		System:    system,
		Messages:  []AnthropicMessage{{Role: "user", Content: "hi"}},
	}

	oai, _ := ToOpenAIRequest(req, "", "", nil)
	if len(oai.Messages) < 1 {
		t.Fatal("expected at least 1 message")
	}
	if oai.Messages[0].Role != "system" {
		t.Errorf("first message role = %q, want system", oai.Messages[0].Role)
	}
	if oai.Messages[0].Content != "System part 1System part 2" {
		t.Errorf("system content = %q, want concatenated text", oai.Messages[0].Content)
	}
}

func TestToOpenAIRequest_TemperatureAndTopP(t *testing.T) {
	temp := 0.7
	topP := 0.9
	req := &AnthropicRequest{
		Model:       "claude-3",
		MaxTokens:   50,
		Temperature: &temp,
		TopP:        &topP,
		Messages:    []AnthropicMessage{{Role: "user", Content: "hi"}},
	}

	oai, _ := ToOpenAIRequest(req, "", "", nil)
	if oai.Temperature == nil || *oai.Temperature != 0.7 {
		t.Errorf("Temperature = %v, want 0.7", oai.Temperature)
	}
	if oai.TopP == nil || *oai.TopP != 0.9 {
		t.Errorf("TopP = %v, want 0.9", oai.TopP)
	}
}

func TestToOpenAIRequest_StopSequences(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "claude-3",
		MaxTokens: 50,
		StopSeq:   []string{"STOP", "END"},
		Messages:  []AnthropicMessage{{Role: "user", Content: "hi"}},
	}

	oai, _ := ToOpenAIRequest(req, "", "", nil)
	if len(oai.Stop) != 2 || oai.Stop[0] != "STOP" || oai.Stop[1] != "END" {
		t.Errorf("Stop = %v, want [STOP END]", oai.Stop)
	}
}

func TestToOpenAIRequest_StreamFlag(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "claude-3",
		MaxTokens: 50,
		Stream:    true,
		Messages:  []AnthropicMessage{{Role: "user", Content: "hi"}},
	}

	oai, _ := ToOpenAIRequest(req, "", "", nil)
	if !oai.Stream {
		t.Error("Stream should be true")
	}
}

func TestToOpenAIRequest_Tools(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "claude-3",
		MaxTokens: 50,
		Messages:  []AnthropicMessage{{Role: "user", Content: "hi"}},
		Tools: []AnthropicTool{
			{
				Name:        "get_weather",
				Description: "Get current weather",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"location": map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	oai, _ := ToOpenAIRequest(req, "", "", nil)
	if len(oai.Tools) != 1 {
		t.Fatalf("Tools length = %d, want 1", len(oai.Tools))
	}
	if oai.Tools[0].Type != "function" {
		t.Errorf("Tools[0].Type = %q, want function", oai.Tools[0].Type)
	}
	if oai.Tools[0].Function.Name != "get_weather" {
		t.Errorf("Tools[0].Function.Name = %q, want get_weather", oai.Tools[0].Function.Name)
	}
	if oai.Tools[0].Function.Description != "Get current weather" {
		t.Errorf("Tools[0].Function.Description = %q, want 'Get current weather'", oai.Tools[0].Function.Description)
	}
}

func TestToOpenAIRequest_UserContentBlocks(t *testing.T) {
	// User message with []any content blocks (text blocks)
	content := []any{
		map[string]any{"type": "text", "text": "Hello "},
		map[string]any{"type": "text", "text": "World"},
	}
	req := &AnthropicRequest{
		Model:     "claude-3",
		MaxTokens: 50,
		Messages:  []AnthropicMessage{{Role: "user", Content: content}},
	}

	oai, _ := ToOpenAIRequest(req, "", "", nil)
	if len(oai.Messages) != 1 {
		t.Fatalf("Messages length = %d, want 1", len(oai.Messages))
	}
	if oai.Messages[0].Content != "Hello \nWorld" {
		t.Errorf("Content = %q, want 'Hello \\nWorld'", oai.Messages[0].Content)
	}
}

func TestToOpenAIRequest_UserToolResult(t *testing.T) {
	content := []any{
		map[string]any{"type": "text", "text": "Before tool result"},
		map[string]any{
			"type":        "tool_result",
			"tool_use_id": "toolu_123",
			"content":     "Tool output here",
		},
		map[string]any{"type": "text", "text": "After tool result"},
	}
	req := &AnthropicRequest{
		Model:     "claude-3",
		MaxTokens: 50,
		Messages:  []AnthropicMessage{{Role: "user", Content: content}},
	}

	oai, _ := ToOpenAIRequest(req, "", "", nil)

	// Expect: user(text), tool(result), user(text)
	if len(oai.Messages) != 3 {
		t.Fatalf("Messages length = %d, want 3", len(oai.Messages))
	}

	if oai.Messages[0].Role != "user" || oai.Messages[0].Content != "Before tool result" {
		t.Errorf("Messages[0] = %+v, want user/'Before tool result'", oai.Messages[0])
	}
	if oai.Messages[1].Role != "tool" || oai.Messages[1].ToolCallID != "toolu_123" || oai.Messages[1].Content != "Tool output here" {
		t.Errorf("Messages[1] = %+v, want tool/toolu_123", oai.Messages[1])
	}
	if oai.Messages[2].Role != "user" || oai.Messages[2].Content != "After tool result" {
		t.Errorf("Messages[2] = %+v, want user/'After tool result'", oai.Messages[2])
	}
}

func TestToOpenAIRequest_AssistantStringContent(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "claude-3",
		MaxTokens: 50,
		Messages: []AnthropicMessage{
			{Role: "user", Content: "hi"},
			{Role: "assistant", Content: "Hello there!"},
		},
	}

	oai, _ := ToOpenAIRequest(req, "", "", nil)
	if len(oai.Messages) != 2 {
		t.Fatalf("Messages length = %d, want 2", len(oai.Messages))
	}
	if oai.Messages[1].Role != "assistant" || oai.Messages[1].Content != "Hello there!" {
		t.Errorf("Messages[1] = %+v", oai.Messages[1])
	}
}

func TestToOpenAIRequest_AssistantToolUse(t *testing.T) {
	content := []any{
		map[string]any{"type": "text", "text": "Let me check the weather."},
		map[string]any{
			"type":  "tool_use",
			"id":    "toolu_456",
			"name":  "get_weather",
			"input": map[string]any{"location": "NYC"},
		},
	}
	req := &AnthropicRequest{
		Model:     "claude-3",
		MaxTokens: 50,
		Messages: []AnthropicMessage{
			{Role: "user", Content: "What's the weather?"},
			{Role: "assistant", Content: content},
		},
	}

	oai, _ := ToOpenAIRequest(req, "", "", nil)
	if len(oai.Messages) != 2 {
		t.Fatalf("Messages length = %d, want 2", len(oai.Messages))
	}

	assistantMsg := oai.Messages[1]
	if assistantMsg.Role != "assistant" {
		t.Errorf("Role = %q, want assistant", assistantMsg.Role)
	}
	if assistantMsg.Content != "Let me check the weather." {
		t.Errorf("Content = %q, want 'Let me check the weather.'", assistantMsg.Content)
	}
	if len(assistantMsg.ToolCalls) != 1 {
		t.Fatalf("ToolCalls length = %d, want 1", len(assistantMsg.ToolCalls))
	}
	if assistantMsg.ToolCalls[0].ID != "toolu_456" {
		t.Errorf("ToolCall ID = %q, want toolu_456", assistantMsg.ToolCalls[0].ID)
	}
	if assistantMsg.ToolCalls[0].Type != "function" {
		t.Errorf("ToolCall Type = %q, want function", assistantMsg.ToolCalls[0].Type)
	}
	if assistantMsg.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("ToolCall Function.Name = %q, want get_weather", assistantMsg.ToolCalls[0].Function.Name)
	}

	// Verify arguments contain JSON
	var args map[string]any
	if err := json.Unmarshal([]byte(assistantMsg.ToolCalls[0].Function.Arguments), &args); err != nil {
		t.Fatalf("failed to parse arguments: %v", err)
	}
	if args["location"] != "NYC" {
		t.Errorf("args[location] = %v, want NYC", args["location"])
	}
}

// ---------------------------------------------------------------------------
// ToAnthropicResponse
// ---------------------------------------------------------------------------

func TestToAnthropicResponse_TextContent(t *testing.T) {
	finishReason := "stop"
	oaiResp := &OpenAIResponse{
		ID:    "chatcmpl-abc",
		Model: "gpt-4",
		Choices: []OpenAIChoice{
			{
				Index:        0,
				Message:      OpenAIMessage{Role: "assistant", Content: "Hi there!"},
				FinishReason: &finishReason,
			},
		},
		Usage: &OpenAIUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}

	resp := ToAnthropicResponse(oaiResp)

	if resp.Type != "message" {
		t.Errorf("Type = %q, want message", resp.Type)
	}
	if resp.Role != "assistant" {
		t.Errorf("Role = %q, want assistant", resp.Role)
	}
	if resp.Model != "gpt-4" {
		t.Errorf("Model = %q, want gpt-4", resp.Model)
	}
	if !strings.HasPrefix(resp.ID, "msg_") {
		t.Errorf("ID = %q, want prefix msg_", resp.ID)
	}
	if len(resp.Content) != 1 {
		t.Fatalf("Content length = %d, want 1", len(resp.Content))
	}
	if resp.Content[0].Type != "text" || resp.Content[0].Text != "Hi there!" {
		t.Errorf("Content[0] = %+v", resp.Content[0])
	}
	if resp.StopReason == nil || *resp.StopReason != "end_turn" {
		t.Errorf("StopReason = %v, want end_turn", resp.StopReason)
	}
	if resp.Usage.InputTokens != 10 {
		t.Errorf("InputTokens = %d, want 10", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 5 {
		t.Errorf("OutputTokens = %d, want 5", resp.Usage.OutputTokens)
	}
}

func TestToAnthropicResponse_ToolCalls(t *testing.T) {
	finishReason := "tool_calls"
	oaiResp := &OpenAIResponse{
		Model: "gpt-4",
		Choices: []OpenAIChoice{
			{
				Index: 0,
				Message: OpenAIMessage{
					Role:    "assistant",
					Content: "",
					ToolCalls: []OpenAIToolCall{
						{
							ID:   "call_123",
							Type: "function",
							Function: OpenAIToolCallFunction{
								Name:      "get_weather",
								Arguments: `{"location":"NYC"}`,
							},
						},
					},
				},
				FinishReason: &finishReason,
			},
		},
	}

	resp := ToAnthropicResponse(oaiResp)

	if len(resp.Content) != 1 {
		t.Fatalf("Content length = %d, want 1 (tool_use block only, no empty text)", len(resp.Content))
	}

	block := resp.Content[0]
	if block.Type != "tool_use" {
		t.Errorf("Content[0].Type = %q, want tool_use", block.Type)
	}
	if block.ID != "call_123" {
		t.Errorf("Content[0].ID = %q, want call_123", block.ID)
	}
	if block.Name != "get_weather" {
		t.Errorf("Content[0].Name = %q, want get_weather", block.Name)
	}

	input, ok := block.Input.(map[string]any)
	if !ok {
		t.Fatalf("Input type = %T, want map[string]any", block.Input)
	}
	if input["location"] != "NYC" {
		t.Errorf("Input[location] = %v, want NYC", input["location"])
	}

	if resp.StopReason == nil || *resp.StopReason != "tool_use" {
		t.Errorf("StopReason = %v, want tool_use", resp.StopReason)
	}
}

func TestToAnthropicResponse_TextAndToolCalls(t *testing.T) {
	finishReason := "tool_calls"
	oaiResp := &OpenAIResponse{
		Model: "gpt-4",
		Choices: []OpenAIChoice{
			{
				Index: 0,
				Message: OpenAIMessage{
					Role:    "assistant",
					Content: "Let me look that up.",
					ToolCalls: []OpenAIToolCall{
						{
							ID:   "call_789",
							Type: "function",
							Function: OpenAIToolCallFunction{
								Name:      "search",
								Arguments: `{"query":"test"}`,
							},
						},
					},
				},
				FinishReason: &finishReason,
			},
		},
	}

	resp := ToAnthropicResponse(oaiResp)

	if len(resp.Content) != 2 {
		t.Fatalf("Content length = %d, want 2 (text + tool_use)", len(resp.Content))
	}
	if resp.Content[0].Type != "text" || resp.Content[0].Text != "Let me look that up." {
		t.Errorf("Content[0] = %+v", resp.Content[0])
	}
	if resp.Content[1].Type != "tool_use" || resp.Content[1].Name != "search" {
		t.Errorf("Content[1] = %+v", resp.Content[1])
	}
}

func TestToAnthropicResponse_NoChoices(t *testing.T) {
	oaiResp := &OpenAIResponse{
		Model:   "gpt-4",
		Choices: []OpenAIChoice{},
	}

	resp := ToAnthropicResponse(oaiResp)

	if resp.Content == nil {
		t.Fatal("Content should not be nil")
	}
	if len(resp.Content) != 0 {
		t.Errorf("Content length = %d, want 0", len(resp.Content))
	}
}

func TestToAnthropicResponse_NoUsage(t *testing.T) {
	oaiResp := &OpenAIResponse{
		Model:   "gpt-4",
		Choices: []OpenAIChoice{},
		Usage:   nil,
	}

	resp := ToAnthropicResponse(oaiResp)

	if resp.Usage.InputTokens != 0 || resp.Usage.OutputTokens != 0 {
		t.Errorf("Usage = %+v, want zero values when no usage", resp.Usage)
	}
}

func TestToAnthropicResponse_InvalidToolArguments(t *testing.T) {
	oaiResp := &OpenAIResponse{
		Model: "gpt-4",
		Choices: []OpenAIChoice{
			{
				Message: OpenAIMessage{
					Role: "assistant",
					ToolCalls: []OpenAIToolCall{
						{
							ID:   "call_bad",
							Type: "function",
							Function: OpenAIToolCallFunction{
								Name:      "broken",
								Arguments: "not valid json",
							},
						},
					},
				},
			},
		},
	}

	resp := ToAnthropicResponse(oaiResp)

	if len(resp.Content) != 1 {
		t.Fatalf("Content length = %d, want 1", len(resp.Content))
	}
	// Should fall back to empty map
	input, ok := resp.Content[0].Input.(map[string]any)
	if !ok {
		t.Fatalf("Input type = %T, want map[string]any", resp.Content[0].Input)
	}
	if len(input) != 0 {
		t.Errorf("Input = %v, want empty map for invalid JSON", input)
	}
}

// ---------------------------------------------------------------------------
// NewAnthropicError
// ---------------------------------------------------------------------------

func TestNewAnthropicError(t *testing.T) {
	err := NewAnthropicError("invalid_request_error", "missing field")

	if err.Type != "error" {
		t.Errorf("Type = %q, want error", err.Type)
	}
	if err.Error.Type != "invalid_request_error" {
		t.Errorf("Error.Type = %q, want invalid_request_error", err.Error.Type)
	}
	if err.Error.Message != "missing field" {
		t.Errorf("Error.Message = %q, want 'missing field'", err.Error.Message)
	}
}

func TestNewAnthropicError_JSONRoundTrip(t *testing.T) {
	err := NewAnthropicError("api_error", "internal error")

	data, jsonErr := json.Marshal(err)
	if jsonErr != nil {
		t.Fatalf("json.Marshal error: %v", jsonErr)
	}

	var decoded AnthropicError
	if jsonErr := json.Unmarshal(data, &decoded); jsonErr != nil {
		t.Fatalf("json.Unmarshal error: %v", jsonErr)
	}

	if decoded.Type != "error" || decoded.Error.Type != "api_error" || decoded.Error.Message != "internal error" {
		t.Errorf("decoded = %+v, want matching fields", decoded)
	}
}

// ---------------------------------------------------------------------------
// extractTextContent
// ---------------------------------------------------------------------------

func TestExtractTextContent_String(t *testing.T) {
	got := extractTextContent("hello world")
	if got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestExtractTextContent_SliceAny(t *testing.T) {
	input := []any{
		map[string]any{"type": "text", "text": "part1"},
		map[string]any{"type": "text", "text": "part2"},
		map[string]any{"type": "image", "url": "http://example.com"}, // non-text, should be ignored
	}
	got := extractTextContent(input)
	if got != "part1part2" {
		t.Errorf("got %q, want %q", got, "part1part2")
	}
}

func TestExtractTextContent_ContentBlocks(t *testing.T) {
	input := []ContentBlock{
		{Type: "text", Text: "block1"},
		{Type: "tool_use", Name: "test"},
		{Type: "text", Text: "block2"},
	}
	got := extractTextContent(input)
	if got != "block1block2" {
		t.Errorf("got %q, want %q", got, "block1block2")
	}
}

func TestExtractTextContent_OtherType(t *testing.T) {
	got := extractTextContent(42)
	if got != "42" {
		t.Errorf("got %q, want %q", got, "42")
	}
}

func TestExtractTextContent_EmptySlice(t *testing.T) {
	got := extractTextContent([]any{})
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

// ---------------------------------------------------------------------------
// mapFinishReason
// ---------------------------------------------------------------------------

func TestMapFinishReason(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"stop", "end_turn"},
		{"length", "max_tokens"},
		{"content_filter", "end_turn"},
		{"tool_calls", "tool_use"},
		{"unknown_reason", "end_turn"},
		{"", "end_turn"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapFinishReason(tt.input)
			if got != tt.want {
				t.Errorf("mapFinishReason(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// generateMessageID
// ---------------------------------------------------------------------------

func TestGenerateMessageID(t *testing.T) {
	id1 := generateMessageID()
	id2 := generateMessageID()

	if !strings.HasPrefix(id1, "msg_") {
		t.Errorf("id1 = %q, want prefix msg_", id1)
	}
	if !strings.HasPrefix(id2, "msg_") {
		t.Errorf("id2 = %q, want prefix msg_", id2)
	}
	// Should be unique
	if id1 == id2 {
		t.Errorf("two generated IDs are identical: %q", id1)
	}
	// msg_ + 24 hex chars = 28 chars total
	if len(id1) != 28 {
		t.Errorf("id length = %d, want 28", len(id1))
	}
}

// ---------------------------------------------------------------------------
// BuildMessageStartEvent
// ---------------------------------------------------------------------------

func TestBuildMessageStartEvent(t *testing.T) {
	msg, evt := BuildMessageStartEvent("gpt-4")

	// Verify the message
	if msg.Type != "message" {
		t.Errorf("msg.Type = %q, want message", msg.Type)
	}
	if msg.Role != "assistant" {
		t.Errorf("msg.Role = %q, want assistant", msg.Role)
	}
	if msg.Model != "gpt-4" {
		t.Errorf("msg.Model = %q, want gpt-4", msg.Model)
	}
	if !strings.HasPrefix(msg.ID, "msg_") {
		t.Errorf("msg.ID = %q, want prefix msg_", msg.ID)
	}
	if msg.Content == nil || len(msg.Content) != 0 {
		t.Errorf("msg.Content = %v, want empty slice", msg.Content)
	}

	// Verify the event
	if evt.Event != "message_start" {
		t.Errorf("evt.Event = %q, want message_start", evt.Event)
	}

	startData, ok := evt.Data.(MessageStartEvent)
	if !ok {
		t.Fatalf("evt.Data type = %T, want MessageStartEvent", evt.Data)
	}
	if startData.Type != "message_start" {
		t.Errorf("startData.Type = %q, want message_start", startData.Type)
	}
	if startData.Message != msg {
		t.Error("startData.Message should reference the same message")
	}
}

// ---------------------------------------------------------------------------
// BuildStreamStopEvents
// ---------------------------------------------------------------------------

func TestBuildStreamStopEvents_WithUsageAndFinishReason(t *testing.T) {
	usage := &OpenAIUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30}
	reason := "stop"
	events := BuildStreamStopEvents(usage, &reason)

	if len(events) != 2 {
		t.Fatalf("events length = %d, want 2", len(events))
	}

	// First event: message_delta
	if events[0].Event != "message_delta" {
		t.Errorf("events[0].Event = %q, want message_delta", events[0].Event)
	}
	deltaData, ok := events[0].Data.(MessageDeltaEvent)
	if !ok {
		t.Fatalf("events[0].Data type = %T, want MessageDeltaEvent", events[0].Data)
	}
	if deltaData.Type != "message_delta" {
		t.Errorf("deltaData.Type = %q, want message_delta", deltaData.Type)
	}
	if deltaData.Delta.StopReason == nil || *deltaData.Delta.StopReason != "end_turn" {
		t.Errorf("StopReason = %v, want end_turn", deltaData.Delta.StopReason)
	}
	if deltaData.Usage == nil {
		t.Fatal("Usage should not be nil")
	}
	if deltaData.Usage.InputTokens != 10 || deltaData.Usage.OutputTokens != 20 {
		t.Errorf("Usage = %+v, want 10/20", deltaData.Usage)
	}

	// Second event: message_stop
	if events[1].Event != "message_stop" {
		t.Errorf("events[1].Event = %q, want message_stop", events[1].Event)
	}
	stopData, ok := events[1].Data.(MessageStopEvent)
	if !ok {
		t.Fatalf("events[1].Data type = %T, want MessageStopEvent", events[1].Data)
	}
	if stopData.Type != "message_stop" {
		t.Errorf("stopData.Type = %q, want message_stop", stopData.Type)
	}
}

func TestBuildStreamStopEvents_NilUsageAndReason(t *testing.T) {
	events := BuildStreamStopEvents(nil, nil)

	if len(events) != 2 {
		t.Fatalf("events length = %d, want 2", len(events))
	}

	deltaData, ok := events[0].Data.(MessageDeltaEvent)
	if !ok {
		t.Fatalf("events[0].Data type = %T, want MessageDeltaEvent", events[0].Data)
	}
	if deltaData.Delta.StopReason == nil || *deltaData.Delta.StopReason != "end_turn" {
		t.Errorf("StopReason = %v, want end_turn (default)", deltaData.Delta.StopReason)
	}
	if deltaData.Usage != nil {
		t.Errorf("Usage = %v, want nil", deltaData.Usage)
	}
}

func TestBuildStreamStopEvents_ToolCallFinishReason(t *testing.T) {
	reason := "tool_calls"
	events := BuildStreamStopEvents(nil, &reason)

	deltaData := events[0].Data.(MessageDeltaEvent)
	if deltaData.Delta.StopReason == nil || *deltaData.Delta.StopReason != "tool_use" {
		t.Errorf("StopReason = %v, want tool_use", deltaData.Delta.StopReason)
	}
}

// ---------------------------------------------------------------------------
// JSON round-trip for Anthropic types
// ---------------------------------------------------------------------------

func TestAnthropicRequest_JSONRoundTrip(t *testing.T) {
	raw := `{
		"model": "claude-3",
		"max_tokens": 100,
		"messages": [{"role": "user", "content": "hello"}],
		"stream": true,
		"stop_sequences": ["STOP"]
	}`

	var req AnthropicRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if req.Model != "claude-3" {
		t.Errorf("Model = %q, want claude-3", req.Model)
	}
	if req.MaxTokens != 100 {
		t.Errorf("MaxTokens = %d, want 100", req.MaxTokens)
	}
	if !req.Stream {
		t.Error("Stream should be true")
	}
	if len(req.StopSeq) != 1 || req.StopSeq[0] != "STOP" {
		t.Errorf("StopSeq = %v, want [STOP]", req.StopSeq)
	}
}

func TestOpenAIResponse_JSONRoundTrip(t *testing.T) {
	raw := `{
		"id": "chatcmpl-1",
		"object": "chat.completion",
		"created": 1234567890,
		"model": "gpt-4",
		"choices": [{
			"index": 0,
			"message": {"role": "assistant", "content": "hi"},
			"finish_reason": "stop"
		}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
	}`

	var resp OpenAIResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if resp.ID != "chatcmpl-1" {
		t.Errorf("ID = %q", resp.ID)
	}
	if resp.Model != "gpt-4" {
		t.Errorf("Model = %q", resp.Model)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("Choices length = %d", len(resp.Choices))
	}
	if resp.Usage == nil || resp.Usage.TotalTokens != 15 {
		t.Errorf("Usage = %+v", resp.Usage)
	}
}
