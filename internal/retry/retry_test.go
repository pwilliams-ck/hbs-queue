package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDoSucceedsFirstAttempt(t *testing.T) {
	t.Parallel()

	calls := 0
	err := Do(context.Background(), 3, func() error {
		calls++
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("got %d calls, want 1", calls)
	}
}

func TestDoRetriesThenSucceeds(t *testing.T) {
	t.Parallel()

	calls := 0
	err := Do(context.Background(), 5, func() error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Errorf("got %d calls, want 3", calls)
	}
}

func TestDoReturnsLastError(t *testing.T) {
	t.Parallel()

	calls := 0
	err := Do(context.Background(), 3, func() error {
		calls++
		return errors.New("always fails")
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "always fails" {
		t.Errorf("got error %q, want %q", err, "always fails")
	}
	if calls != 3 {
		t.Errorf("got %d calls, want 3", calls)
	}
}

func TestDoRespectsContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	calls := 0
	go func() {
		// Cancel after a short delay to let the first attempt run.
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := Do(ctx, 10, func() error {
		calls++
		return errors.New("keeps failing")
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled in error chain, got: %v", err)
	}
	// Should have stopped early, not all 10 attempts.
	if calls >= 10 {
		t.Errorf("expected fewer than 10 calls, got %d", calls)
	}
}
