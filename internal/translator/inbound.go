package translator

import (
	"encoding/json"
	"fmt"

	"github.com/fajarhide/heimsense/internal/adapter"
	"github.com/fajarhide/heimsense/internal/config"
)

// InboundEngine translates incoming requests from various formats into
// the standard OpenAIRequest format used internally by HeimSense.
type InboundEngine struct {
	cfg *config.Config
}

func NewInboundEngine(cfg *config.Config) *InboundEngine {
	return &InboundEngine{cfg: cfg}
}

// Translate takes a raw JSON payload and its detected format,
// and returns a normalized adapter.OpenAIRequest.
func (e *InboundEngine) Translate(rawPayload []byte, format FormatType) (*adapter.OpenAIRequest, error) {
	switch format {
	case FormatOpenAI:
		return e.fromOpenAI(rawPayload)
	case FormatAnthropic:
		return e.fromAnthropic(rawPayload)
	default:
		// Fallback to OpenAI if unknown
		return e.fromOpenAI(rawPayload)
	}
}

func (e *InboundEngine) fromOpenAI(raw []byte) (*adapter.OpenAIRequest, error) {
	var req adapter.OpenAIRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI request: %w", err)
	}

	// Apply model overrides if configured
	if e.cfg != nil {
		if e.cfg.ForceModel != "" {
			req.Model = e.cfg.ForceModel
		} else if req.Model == "" {
			req.Model = e.cfg.DefaultModel
		}
	}

	return &req, nil
}

func (e *InboundEngine) fromAnthropic(raw []byte) (*adapter.OpenAIRequest, error) {
	var anthReq adapter.AnthropicRequest
	if err := json.Unmarshal(raw, &anthReq); err != nil {
		return nil, fmt.Errorf("failed to parse Anthropic request: %w", err)
	}

	return adapter.ToOpenAIRequest(&anthReq, e.cfg.DefaultModel, e.cfg.ForceModel, e.cfg)
}
