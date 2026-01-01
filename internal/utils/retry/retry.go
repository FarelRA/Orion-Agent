package retry

import (
	"context"
	"math"
	"time"
)

// Config holds retry configuration.
type Config struct {
	MaxAttempts int
	InitialWait time.Duration
	MaxWait     time.Duration
	Multiplier  float64
}

// DefaultConfig returns sensible retry defaults.
func DefaultConfig() Config {
	return Config{
		MaxAttempts: 3,
		InitialWait: 100 * time.Millisecond,
		MaxWait:     10 * time.Second,
		Multiplier:  2.0,
	}
}

// Do executes fn with retry logic using default config.
func Do[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	return DoWithConfig(ctx, DefaultConfig(), fn)
}

// DoWithConfig executes fn with retry logic using provided config.
func DoWithConfig[T any](ctx context.Context, cfg Config, fn func() (T, error)) (T, error) {
	var result T
	var err error

	wait := cfg.InitialWait

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		result, err = fn()
		if err == nil {
			return result, nil
		}

		// Don't wait after the last attempt
		if attempt == cfg.MaxAttempts {
			break
		}

		// Check context before waiting
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(wait):
		}

		// Exponential backoff
		wait = time.Duration(float64(wait) * cfg.Multiplier)
		if wait > cfg.MaxWait {
			wait = cfg.MaxWait
		}
	}

	return result, err
}

// DoSimple executes fn with retry logic, returning only error.
func DoSimple(ctx context.Context, maxAttempts int, fn func() error) error {
	cfg := DefaultConfig()
	cfg.MaxAttempts = maxAttempts

	_, err := DoWithConfig(ctx, cfg, func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}

// DoWithBackoff executes with custom backoff calculation.
func DoWithBackoff(ctx context.Context, maxAttempts int, backoffFn func(attempt int) time.Duration, fn func() error) error {
	var err error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err = fn()
		if err == nil {
			return nil
		}

		if attempt == maxAttempts {
			break
		}

		wait := backoffFn(attempt)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}

	return err
}

// ExponentialBackoff returns a backoff function with exponential growth.
func ExponentialBackoff(initial time.Duration, max time.Duration) func(int) time.Duration {
	return func(attempt int) time.Duration {
		wait := time.Duration(float64(initial) * math.Pow(2, float64(attempt-1)))
		if wait > max {
			return max
		}
		return wait
	}
}
