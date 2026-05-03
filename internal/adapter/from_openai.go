package adapter

// FromOpenAIRequest converts an internal OpenAI chat completion request into an Anthropic request.
// This is used when routing an OpenAI-formatted request to a native Anthropic provider.
func FromOpenAIRequest(req *OpenAIRequest) *AnthropicRequest {
	anthReq := &AnthropicRequest{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
		StopSeq:     req.Stop,
	}

	if anthReq.MaxTokens == 0 {
		anthReq.MaxTokens = 4096 // Anthropic requires max_tokens
	}

	// Tools
	if len(req.Tools) > 0 {
		anthReq.Tools = make([]AnthropicTool, len(req.Tools))
		for i, t := range req.Tools {
			anthReq.Tools[i] = AnthropicTool{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				InputSchema: t.Function.Parameters,
			}
		}
	}

	// Messages
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			anthReq.System = msg.Content
			continue
		}

		anthMsg := AnthropicMessage{
			Role: msg.Role,
		}

		// Tool calls (Assistant invoking a tool)
		if len(msg.ToolCalls) > 0 {
			var blocks []ContentBlock
			if msg.Content != "" {
				blocks = append(blocks, ContentBlock{
					Type: "text",
					Text: msg.Content,
				})
			}
			for _, tc := range msg.ToolCalls {
				// Convert JSON arguments back to object for Anthropic
				// In a full implementation, we'd unmarshal arguments, but for raw proxying
				// Anthropic expects an object. We'll leave it as any, or map.
				blocks = append(blocks, ContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: tc.Function.Arguments, // Note: Anthropic might want unmarshaled JSON here depending on strictness
				})
			}
			anthMsg.Content = blocks
		} else if msg.Role == "tool" {
			// Tool result (User providing result to Assistant)
			anthMsg.Role = "user"
			anthMsg.Content = []ContentBlock{
				{
					Type:      "tool_result",
					ToolUseID: msg.ToolCallID,
					Content:   msg.Content,
				},
			}
		} else {
			// Normal text message
			anthMsg.Content = msg.Content
		}

		anthReq.Messages = append(anthReq.Messages, anthMsg)
	}

	return anthReq
}
