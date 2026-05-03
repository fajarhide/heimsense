package translator

import (
	"encoding/json"
	"strings"
)

// FormatType represents the structure of an AI request/response.
type FormatType string

const (
	FormatOpenAI    FormatType = "openai"
	FormatAnthropic FormatType = "anthropic"
	FormatUnknown   FormatType = "unknown"
)

// DetectFormat analyzes the HTTP endpoint path and raw JSON payload to determine
// the format of the incoming request.
func DetectFormat(endpoint string, rawPayload []byte) FormatType {
	// 1. Endpoint heuristics
	if strings.HasSuffix(endpoint, "/v1/messages") || strings.HasSuffix(endpoint, "/v1/complete") {
		return FormatAnthropic
	}
	if strings.HasSuffix(endpoint, "/v1/chat/completions") || strings.HasSuffix(endpoint, "/v1/completions") {
		return FormatOpenAI
	}

	// 2. Payload heuristics (if endpoint is ambiguous)
	if len(rawPayload) == 0 {
		return FormatUnknown
	}

	var parsed map[string]any
	if err := json.Unmarshal(rawPayload, &parsed); err != nil {
		return FormatUnknown
	}

	// OpenAI typically has "messages" array containing "role" and "content" (string/array)
	// Anthropic also has "messages", but has "system" at the root level, and "max_tokens" is required.

	// Check for Anthropic specific root keys
	if _, hasSystem := parsed["system"]; hasSystem {
		if _, isString := parsed["system"].(string); isString || isArray(parsed["system"]) {
			return FormatAnthropic
		}
	}
	if _, hasMaxTokens := parsed["max_tokens"]; hasMaxTokens {
		// OpenAI uses max_tokens too, but Anthropic makes it strictly required at the root
		// Let's check for OpenAI specific keys
		if _, hasResponseFormat := parsed["response_format"]; hasResponseFormat {
			return FormatOpenAI
		}
		if _, hasSeed := parsed["seed"]; hasSeed {
			return FormatOpenAI
		}
	}

	// Deep dive into messages array
	if messages, ok := parsed["messages"].([]any); ok && len(messages) > 0 {
		if firstMsg, ok := messages[0].(map[string]any); ok {
			// If role is system in the array, it's OpenAI (Anthropic puts it at root)
			if role, _ := firstMsg["role"].(string); role == "system" {
				return FormatOpenAI
			}

			// If content has tool_calls or function_call, it's OpenAI
			if _, hasToolCalls := firstMsg["tool_calls"]; hasToolCalls {
				return FormatOpenAI
			}
			if _, hasFunctionCall := firstMsg["function_call"]; hasFunctionCall {
				return FormatOpenAI
			}
		}
	}

	// If we can't be sure, default to OpenAI as it's the industry standard
	return FormatOpenAI
}

func isArray(v any) bool {
	_, ok := v.([]any)
	return ok
}
