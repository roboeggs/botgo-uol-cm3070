package handlers

import (
	"testing"
	"time"

	tele "gopkg.in/telebot.v4"
)

// --- rateLimiter ---

func TestRateLimiter_FirstCallAllowed(t *testing.T) {
	rl := newRateLimiter(time.Minute)
	if !rl.allow(1) {
		t.Fatal("first call denied")
	}
}

func TestRateLimiter_SecondCallWithinWindowDenied(t *testing.T) {
	rl := newRateLimiter(time.Minute)
	rl.allow(1)
	if rl.allow(1) {
		t.Fatal("second call within window allowed")
	}
}

func TestRateLimiter_DifferentUsersIndependent(t *testing.T) {
	rl := newRateLimiter(time.Minute)
	if !rl.allow(1) || !rl.allow(2) {
		t.Fatal("first calls for different users denied")
	}
	if rl.allow(1) || rl.allow(2) {
		t.Fatal("second calls for different users allowed")
	}
}

func TestRateLimiter_AfterWindowAllowed(t *testing.T) {
	rl := newRateLimiter(50 * time.Millisecond)
	rl.allow(1)
	time.Sleep(60 * time.Millisecond)
	if !rl.allow(1) {
		t.Fatal("call after window denied")
	}
}

func TestRateLimiter_LazyCleanupRemovesStale(t *testing.T) {
	// window=10ms → cleanupInterval = 100ms.
	rl := newRateLimiter(10 * time.Millisecond)

	rl.allow(1) // triggers cleanup (first call), cleaned=now
	rl.allow(2)

	// Wait until both entries become stale and cleanup interval elapses.
	time.Sleep(120 * time.Millisecond)

	rl.allow(3) // cleanup should now remove 1 and 2

	rl.mu.Lock()
	size := len(rl.seen)
	rl.mu.Unlock()

	if size != 1 {
		t.Errorf("expected 1 entry after cleanup, got %d", size)
	}
}

// --- MaxWords ---

// fakeContext — minimal mock of tele.Context.
// The embedded interface is nil; we override only the methods
// actually invoked by the middleware.
type fakeContext struct {
	tele.Context
	userID int64
	text   string
	sent   []string
}

func (f *fakeContext) Sender() *tele.User { return &tele.User{ID: f.userID} }
func (f *fakeContext) Chat() *tele.Chat   { return &tele.Chat{ID: f.userID} }
func (f *fakeContext) Text() string       { return f.text }
func (f *fakeContext) Send(what interface{}, _ ...interface{}) error {
	if s, ok := what.(string); ok {
		f.sent = append(f.sent, s)
	}
	return nil
}

func TestMaxWords_UnderLimit(t *testing.T) {
	called := false
	next := func(c tele.Context) error {
		called = true
		return nil
	}

	handler := MaxWords(10)(next)
	c := &fakeContext{userID: 1, text: "one two three"}

	if err := handler(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("next handler was not called")
	}
}

func TestMaxWords_AtLimit(t *testing.T) {
	called := false
	next := func(c tele.Context) error {
		called = true
		return nil
	}

	handler := MaxWords(3)(next)
	c := &fakeContext{userID: 1, text: "one two three"}

	if err := handler(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("next handler should be called when exactly at limit")
	}
}

func TestMaxWords_OverLimit(t *testing.T) {
	called := false
	next := func(c tele.Context) error {
		called = true
		return nil
	}

	handler := MaxWords(3)(next)
	c := &fakeContext{userID: 1, text: "one two three four"}

	if err := handler(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("next handler should not be called when over limit")
	}
	if len(c.sent) == 0 {
		t.Error("expected an error reply to the user")
	}
}

func TestMaxWords_EmptyText(t *testing.T) {
	// Empty text (e.g. a callback without text) must not trigger an error.
	called := false
	next := func(c tele.Context) error {
		called = true
		return nil
	}

	handler := MaxWords(3)(next)
	c := &fakeContext{userID: 1, text: ""}

	if err := handler(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("next handler should be called for empty text")
	}
}