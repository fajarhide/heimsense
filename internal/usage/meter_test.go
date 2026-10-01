package usage

import (
	"sync"
	"testing"
	"time"
)

// fixedClock lets a test roll the window without sleeping.
type fixedClock struct{ t time.Time }

func (c *fixedClock) now() time.Time      { return c.t }
func (c *fixedClock) add(d time.Duration) { c.t = c.t.Add(d) }

func newTestMeter(limits Limits) (*Meter, *fixedClock) {
	clock := &fixedClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	m := New(limits)
	m.now = clock.now
	m.windowStart = clock.t
	return m, clock
}

func TestNew_DefaultsTheWindow(t *testing.T) {
	if got := New(Limits{}).limits.Window; got != DefaultWindow {
		t.Errorf("Window = %v, want %v", got, DefaultWindow)
	}
}

func TestAllow_NoLimitsStillCounts(t *testing.T) {
	m, _ := newTestMeter(Limits{Window: time.Hour})

	for i := 0; i < 5; i++ {
		if err := m.Allow(); err != nil {
			t.Fatalf("Allow() #%d = %v, want nil with no ceiling set", i, err)
		}
	}
	m.AddTokens(100, 50)

	snap := m.Snapshot()
	if snap.Requests != 5 {
		t.Errorf("Requests = %d, want 5", snap.Requests)
	}
	if snap.Tokens != 150 {
		t.Errorf("Tokens = %d, want 150", snap.Tokens)
	}
}

func TestAllow_RefusesAtRequestCeiling(t *testing.T) {
	m, _ := newTestMeter(Limits{Window: time.Hour, MaxRequests: 2})

	if err := m.Allow(); err != nil {
		t.Fatalf("first Allow() = %v, want nil", err)
	}
	if err := m.Allow(); err != nil {
		t.Fatalf("second Allow() = %v, want nil", err)
	}

	err := m.Allow()
	exceeded, ok := err.(*Exceeded)
	if !ok {
		t.Fatalf("third Allow() = %v, want *Exceeded", err)
	}
	if exceeded.Limit != "request" {
		t.Errorf("Limit = %q, want %q", exceeded.Limit, "request")
	}
	if exceeded.RetryAfter <= 0 {
		t.Errorf("RetryAfter = %v, want a positive duration", exceeded.RetryAfter)
	}

	// A refused request must not be counted.
	if got := m.Snapshot().Requests; got != 2 {
		t.Errorf("Requests = %d, want 2, a refused call should not count", got)
	}
}

func TestAllow_RefusesAtTokenCeiling(t *testing.T) {
	m, _ := newTestMeter(Limits{Window: time.Hour, MaxTokens: 100})

	if err := m.Allow(); err != nil {
		t.Fatalf("Allow() = %v, want nil", err)
	}
	m.AddTokens(60, 50) // 110, over the ceiling

	err := m.Allow()
	exceeded, ok := err.(*Exceeded)
	if !ok {
		t.Fatalf("Allow() = %v, want *Exceeded", err)
	}
	if exceeded.Limit != "token" {
		t.Errorf("Limit = %q, want %q", exceeded.Limit, "token")
	}
}

func TestAllow_WindowRollClearsCounters(t *testing.T) {
	m, clock := newTestMeter(Limits{Window: time.Hour, MaxRequests: 1})

	if err := m.Allow(); err != nil {
		t.Fatalf("Allow() = %v, want nil", err)
	}
	if err := m.Allow(); err == nil {
		t.Fatal("Allow() = nil, want the ceiling to refuse inside the window")
	}

	clock.add(time.Hour)

	if err := m.Allow(); err != nil {
		t.Fatalf("Allow() after the window rolled = %v, want nil", err)
	}
	if got := m.Snapshot().Requests; got != 1 {
		t.Errorf("Requests = %d, want 1 in the fresh window", got)
	}
}

func TestMeter_ConcurrentUse(t *testing.T) {
	m, _ := newTestMeter(Limits{Window: time.Hour})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.Allow()
			m.AddTokens(1, 1)
			_ = m.Snapshot()
		}()
	}
	wg.Wait()

	snap := m.Snapshot()
	if snap.Requests != 50 {
		t.Errorf("Requests = %d, want 50", snap.Requests)
	}
	if snap.Tokens != 100 {
		t.Errorf("Tokens = %d, want 100", snap.Tokens)
	}
}
