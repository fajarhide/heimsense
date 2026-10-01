// Package usage counts what Heimsense spends upstream and refuses once a
// ceiling is reached.
//
// Everything is held in memory: counters start empty on every restart and are
// not shared between instances. That is enough to stop an agent stuck in a loop
// from draining an account, and deliberately short of per-key accounting, which
// belongs in a gateway rather than an adapter.
package usage

import (
	"fmt"
	"sync"
	"time"
)

// Limits is the ceiling for a single window. A zero Max means that dimension is
// counted but never refused.
type Limits struct {
	Window      time.Duration
	MaxTokens   int64
	MaxRequests int64
}

// DefaultWindow applies when no window is configured.
const DefaultWindow = time.Hour

// Meter tracks requests and tokens for the current window.
type Meter struct {
	mu          sync.Mutex
	limits      Limits
	windowStart time.Time
	tokens      int64
	requests    int64

	// now is swapped in tests so a window can be rolled without sleeping.
	now func() time.Time
}

// Exceeded says which ceiling was reached and when the window rolls.
type Exceeded struct {
	Limit      string
	RetryAfter time.Duration
}

func (e *Exceeded) Error() string {
	return fmt.Sprintf("%s limit reached for this window, retry in %s", e.Limit, e.RetryAfter.Round(time.Second))
}

// New returns a Meter that counts from now.
func New(limits Limits) *Meter {
	if limits.Window <= 0 {
		limits.Window = DefaultWindow
	}

	m := &Meter{limits: limits, now: time.Now}
	m.windowStart = m.now()
	return m
}

// Allow records one request, or returns *Exceeded and records nothing.
//
// Call it before going upstream. A request is counted on the way in because its
// cost is only known on the way out, so a token ceiling can be passed by at
// most one request. That is accepted: the point is to stop a loop, not to bill.
func (m *Meter) Allow() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rollLocked()

	if m.limits.MaxRequests > 0 && m.requests >= m.limits.MaxRequests {
		return &Exceeded{Limit: "request", RetryAfter: m.retryAfterLocked()}
	}
	if m.limits.MaxTokens > 0 && m.tokens >= m.limits.MaxTokens {
		return &Exceeded{Limit: "token", RetryAfter: m.retryAfterLocked()}
	}

	m.requests++
	return nil
}

// AddTokens records what a completed request actually cost.
func (m *Meter) AddTokens(input, output int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rollLocked()

	m.tokens += int64(input) + int64(output)
}

// Snapshot is the current window, shaped for the /usage response.
type Snapshot struct {
	WindowSeconds    int64 `json:"window_seconds"`
	RemainingSeconds int64 `json:"window_remaining_seconds"`
	Requests         int64 `json:"requests"`
	Tokens           int64 `json:"tokens"`
	MaxRequests      int64 `json:"max_requests"`
	MaxTokens        int64 `json:"max_tokens"`
}

// Snapshot reports the current counters.
func (m *Meter) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rollLocked()

	return Snapshot{
		WindowSeconds:    int64(m.limits.Window.Seconds()),
		RemainingSeconds: int64(m.retryAfterLocked().Seconds()),
		Requests:         m.requests,
		Tokens:           m.tokens,
		MaxRequests:      m.limits.MaxRequests,
		MaxTokens:        m.limits.MaxTokens,
	}
}

// rollLocked starts a fresh window once the current one has elapsed.
//
// ponytail: tumbling window, so a burst straddling the boundary can spend two
// windows back to back. A sliding window would smooth that out and needs a ring
// buffer; move to one if the boundary burst turns out to matter.
func (m *Meter) rollLocked() {
	if m.now().Sub(m.windowStart) < m.limits.Window {
		return
	}

	m.windowStart = m.now()
	m.tokens = 0
	m.requests = 0
}

func (m *Meter) retryAfterLocked() time.Duration {
	elapsed := m.now().Sub(m.windowStart)
	if remaining := m.limits.Window - elapsed; remaining > 0 {
		return remaining
	}
	return 0
}
