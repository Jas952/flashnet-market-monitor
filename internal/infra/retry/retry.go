package retry

// Retry mechanism with exponential backoff and full jitter
// Handles retryable errors (HTTP 429, 500, 502, 503, 504)
// Supports Retry-After header for 429 responses
// Applies random delay (full jitter) to prevent thundering herd

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Options struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	Backoff    float64
}

type HTTPError struct {
	StatusCode int
	Body       []byte
	RetryAfter time.Duration
}

func (e *HTTPError) Error() string {
	if e == nil {
		return "http error: <nil>"
	}
	if len(e.Body) == 0 {
		return fmt.Sprintf("http error (%d)", e.StatusCode)
	}
	return fmt.Sprintf("http error (%d): %s", e.StatusCode, string(e.Body))
}

func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	var he *HTTPError
	if errors.As(err, &he) {
		switch he.StatusCode {
		case 429, 500, 502, 503, 504:
			return true
		default:
			return false
		}
	}
	return false
}

var seedOnce sync.Once

func seedRand() {
	seedOnce.Do(func() { rand.Seed(time.Now().UnixNano()) })
}

func ParseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	layouts := []string{time.RFC1123, time.RFC1123Z, time.RFC850, time.ANSIC}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, v); err == nil {
			d := time.Until(t)
			if d < 0 {
				return 0
			}
			return d
		}
	}
	return 0
}

func clamp(d, max time.Duration) time.Duration {
	if max > 0 && d > max {
		return max
	}
	return d
}

func FullJitterSleep(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	if baseDelay <= 0 {
		return 0
	}
	maxForAttempt := baseDelay << attempt
	maxForAttempt = clamp(maxForAttempt, maxDelay)
	if maxForAttempt <= 0 {
		return 0
	}
	seedRand()
	return time.Duration(rand.Int63n(int64(maxForAttempt) + 1))
}

func Do(ctx context.Context, opts Options, fn func() error) error {
	if opts.MaxRetries < 0 {
		opts.MaxRetries = 0
	}
	if opts.BaseDelay <= 0 {
		opts.BaseDelay = 300 * time.Millisecond
	}
	if opts.Backoff <= 0 {
		opts.Backoff = 2.0
	}
	// If MaxDelay is not set, we still allow growth, but clamp() will no-op.

	totalAttempts := 1 + opts.MaxRetries
	var lastErr error

	for attempt := 0; attempt < totalAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err

		if !IsRetryable(err) || attempt == totalAttempts-1 {
			return lastErr
		}

		// Default: jitter sleep based on exponential cap for this attempt.
		sleep := FullJitterSleep(attempt, opts.BaseDelay, opts.MaxDelay)

		// Prefer Retry-After for 429 if present.
		var he *HTTPError
		if errors.As(err, &he) && he.StatusCode == 429 && he.RetryAfter > 0 {
			sleep = clamp(he.RetryAfter, opts.MaxDelay)
		}

		t := time.NewTimer(sleep)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}

	return lastErr
}
