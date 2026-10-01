package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/fajarhide/heimsense/internal/adapter"
	"github.com/fajarhide/heimsense/internal/usage"
)

// NewUsageHandler reports the current window's counters.
//
// Read only, and the same counters /v1/messages writes, so the numbers that
// enforce a ceiling are the numbers an operator can see.
func NewUsageHandler(meter *usage.Meter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(
				adapter.NewAnthropicError("invalid_request_error", "method not allowed"),
			)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(meter.Snapshot())
	})
}

// writeLimitError answers a refused request with 429 and a Retry-After, so a
// caller backs off instead of retrying straight away.
func (h *MessagesHandler) writeLimitError(w http.ResponseWriter, err error) {
	var exceeded *usage.Exceeded
	if errors.As(err, &exceeded) {
		if secs := int(exceeded.RetryAfter.Seconds()); secs > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(secs))
		}
		h.logger.Warn("request refused by usage ceiling",
			"limit", exceeded.Limit,
			"retry_after", exceeded.RetryAfter.Round(time.Second),
		)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(
		adapter.NewAnthropicError("rate_limit_error", err.Error()),
	)
}
