package telegram

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// noRetry — stub: never retry.
func noRetry(err error) (int, bool) { return 0, false }

func TestSender_DeliversSingleMessage(t *testing.T) {
	got := make(chan string, 1)
	send := func(_ context.Context, _ int64, text string) error {
		got <- text
		return nil
	}

	s := newSender(send, noRetry, 0, 0, 10)
	s.Enqueue(42, "hello")

	select {
	case msg := <-got:
		if msg != "hello" {
			t.Errorf("got %q, want %q", msg, "hello")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: message not delivered")
	}
}

func TestSender_DeliversInOrder(t *testing.T) {
	var calls int32
	done := make(chan struct{})
	var received []string

	send := func(_ context.Context, _ int64, text string) error {
		received = append(received, text)
		if atomic.AddInt32(&calls, 1) == 3 {
			close(done)
		}
		return nil
	}

	s := newSender(send, noRetry, 0, 0, 10)
	s.Enqueue(1, "a")
	s.Enqueue(1, "b")
	s.Enqueue(1, "c")

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}

	want := []string{"a", "b", "c"}
	for i := range want {
		if received[i] != want[i] {
			t.Errorf("order[%d] = %q, want %q", i, received[i], want[i])
		}
	}
}

func TestSender_DropsWhenQueueFull(t *testing.T) {
	block := make(chan struct{})
	send := func(_ context.Context, _ int64, _ string) error {
		<-block
		return nil
	}

	// Small queue: 1 blocked worker + 3 in the buffer.
	s := newSender(send, noRetry, 0, 0, 3)

	// Fill the queue beyond capacity.
	for i := 0; i < 10; i++ {
		s.Enqueue(1, "msg")
	}

	// Enqueue must not block, even when the queue is full.
	done := make(chan struct{})
	go func() {
		s.Enqueue(1, "extra")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Enqueue blocked on full queue")
	}

	close(block)
}

func TestSender_RetriesOnFlood(t *testing.T) {
	var calls int32
	done := make(chan struct{})

	send := func(_ context.Context, _ int64, _ string) error {
		if atomic.AddInt32(&calls, 1) == 1 {
			return errors.New("flood")
		}
		close(done)
		return nil
	}
	retryAfter := func(err error) (int, bool) {
		if err.Error() == "flood" {
			return 0, true // retry immediately, no wait
		}
		return 0, false
	}

	s := newSender(send, retryAfter, 0, 0, 10)
	s.Enqueue(1, "hi")

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("retry did not happen")
	}

	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Errorf("calls = %d, want 2", n)
	}
}

func TestSender_NoRetryOnOtherError(t *testing.T) {
	var calls int32
	second := make(chan struct{})

	send := func(_ context.Context, _ int64, _ string) error {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			return errors.New("network")
		}
		close(second)
		return nil
	}

	s := newSender(send, noRetry, 0, 0, 10)
	s.Enqueue(1, "first")

	// Let the worker process the first (failing) message, then send the second.
	time.Sleep(50 * time.Millisecond)
	s.Enqueue(1, "second")

	select {
	case <-second:
	case <-time.After(2 * time.Second):
		t.Fatal("second message not delivered")
	}

	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Errorf("calls = %d, want 2 (no retry on non-flood)", n)
	}
}