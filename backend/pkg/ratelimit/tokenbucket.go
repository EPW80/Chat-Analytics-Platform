// Package ratelimit provides a small, dependency-free token-bucket limiter
// used to throttle per-connection message rates.
package ratelimit

import (
	"sync"
	"time"
)

// TokenBucket is a thread-safe token-bucket rate limiter. Tokens refill
// continuously at refillPerSec up to capacity (the burst size).
type TokenBucket struct {
	mu           sync.Mutex
	capacity     float64
	tokens       float64
	refillPerSec float64
	last         time.Time

	// now is injectable so tests can drive the clock deterministically.
	now func() time.Time
}

// NewTokenBucket returns a bucket that starts full with the given capacity
// (burst) and refills at refillPerSec tokens per second.
func NewTokenBucket(capacity, refillPerSec float64) *TokenBucket {
	return &TokenBucket{
		capacity:     capacity,
		tokens:       capacity,
		refillPerSec: refillPerSec,
		last:         time.Now(),
		now:          time.Now,
	}
}

// Allow reports whether a single token is available, consuming it if so.
func (b *TokenBucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.now()
	if elapsed := now.Sub(b.last).Seconds(); elapsed > 0 {
		b.tokens += elapsed * b.refillPerSec
		if b.tokens > b.capacity {
			b.tokens = b.capacity
		}
		b.last = now
	}

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}
