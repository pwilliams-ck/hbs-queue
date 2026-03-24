// Package retry provides a shared retry helper with exponential backoff
// and jitter. Used by API clients and workflow steps.
package retry

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
)

// Do calls fn up to maxAttempts times, sleeping with exponential backoff
// and jitter between attempts. It stops early if ctx is cancelled.
// Returns nil on the first successful call, or the last error if all
// attempts fail.
func Do(ctx context.Context, maxAttempts int, fn func() error) error {
	var lastErr error
	for i := range maxAttempts {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		// Don't sleep after the last attempt.
		if i == maxAttempts-1 {
			break
		}

		backoff := baseDelay(i) + jitter(baseDelay(i)/2)

		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w (last error: %w)", ctx.Err(), lastErr)
		case <-time.After(backoff):
		}
	}
	return lastErr
}

// baseDelay returns 1s, 2s, 4s, 8s, ... capped at 30s.
func baseDelay(attempt int) time.Duration {
	d := time.Second << uint(attempt)
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	return d
}

// jitter returns a random duration in [0, max) using crypto/rand.
func jitter(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return max / 2 // fallback
	}
	return time.Duration(n.Int64())
}
