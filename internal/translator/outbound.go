package translator

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fajarhide/heimsense/internal/adapter"
	"github.com/fajarhide/heimsense/internal/config"
)

// OutboundEngine translates the standard OpenAIRequest into the native
// format expected by the chosen upstream provider.
type OutboundEngine struct {
	cfg *config.Config
}

func NewOutboundEngine(cfg *config.Config) *OutboundEngine {
	return &OutboundEngine{cfg: cfg}
}

// Translate converts the standard request to the format expected by the provider.
// Returns the JSON payload to send and the expected format of the response.
func (e *OutboundEngine) Translate(req *adapter.OpenAIRequest, provider config.ProviderConfig) ([]byte, FormatType, error) {
	format := e.detectProviderFormat(provider)

	switch format {
	case FormatOpenAI:
		// Request is already in OpenAI format
		b, err := json.Marshal(req)
		return b, FormatOpenAI, err
	case FormatAnthropic:
		// We need to convert OpenAIRequest -> AnthropicRequest
		anthReq := adapter.FromOpenAIRequest(req)
		b, err := json.Marshal(anthReq)
		return b, FormatAnthropic, err
	default:
		b, err := json.Marshal(req)
		return b, FormatOpenAI, err
	}
}

// TranslateResponse converts the provider's response back to the client's requested format.
func (e *OutboundEngine) TranslateResponse(providerResp []byte, providerFormat FormatType, clientFormat FormatType) ([]byte, error) {
	if providerFormat == clientFormat {
		return providerResp, nil // No translation needed
	}

	if providerFormat == FormatOpenAI && clientFormat == FormatAnthropic {
		var oaiResp adapter.OpenAIResponse
		if err := json.Unmarshal(providerResp, &oaiResp); err != nil {
			return nil, fmt.Errorf("failed to parse provider openai response: %w", err)
		}
		anthResp := adapter.ToAnthropicResponse(&oaiResp)
		return json.Marshal(anthResp)
	}

	if providerFormat == FormatAnthropic && clientFormat == FormatOpenAI {
		// Not implemented yet: ToOpenAIResponse from Anthropic
		// For now, return as is (client might fail, but we'll add this later)
		return providerResp, nil
	}

	return providerResp, nil
}

func (e *OutboundEngine) detectProviderFormat(p config.ProviderConfig) FormatType {
	// Simple heuristic based on BaseURL
	if strings.Contains(p.BaseURL, "api.anthropic.com") {
		return FormatAnthropic
	}
	// Everything else (OpenRouter, DeepSeek, GLM, local Ollama) uses OpenAI
	return FormatOpenAI
}
