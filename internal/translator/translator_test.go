package translator

import (
	"testing"
)

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		payload  string
		want     FormatType
	}{
		{
			name:     "Anthropic endpoint",
			endpoint: "/v1/messages",
			payload:  `{}`,
			want:     FormatAnthropic,
		},
		{
			name:     "OpenAI endpoint",
			endpoint: "/v1/chat/completions",
			payload:  `{}`,
			want:     FormatOpenAI,
		},
		{
			name:     "Anthropic payload system at root",
			endpoint: "/v1/completions_unknown",
			payload:  `{"model": "test", "system": "hello", "messages": []}`,
			want:     FormatAnthropic,
		},
		{
			name:     "OpenAI payload system in messages",
			endpoint: "/v1/completions_unknown",
			payload:  `{"model": "test", "messages": [{"role": "system", "content": "hello"}]}`,
			want:     FormatOpenAI,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFormat(tt.endpoint, []byte(tt.payload))
			if got != tt.want {
				t.Errorf("DetectFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInboundEngine_TranslateOpenAI(t *testing.T) {
	// A simple OpenAI request
	payload := `{
		"model": "gpt-4",
		"messages": [{"role": "user", "content": "hello"}]
	}`

	engine := NewInboundEngine(nil)
	req, err := engine.Translate([]byte(payload), FormatOpenAI)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.Model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", req.Model)
	}
	if len(req.Messages) != 1 || req.Messages[0].Content != "hello" {
		t.Errorf("unexpected messages: %+v", req.Messages)
	}
}

func TestOutboundEngine_TranslateToAnthropic(t *testing.T) {
	payload := `{
		"model": "gpt-4",
		"messages": [
			{"role": "system", "content": "you are a bot"},
			{"role": "user", "content": "hello"}
		]
	}`

	inEngine := NewInboundEngine(nil)
	_, _ = inEngine.Translate([]byte(payload), FormatOpenAI)

	// Assume we detected the provider is anthropic
	// outEngine := NewOutboundEngine(nil)
	// (Test deferred since OutboundEngine detects format by ProviderConfig BaseURL right now)
}
