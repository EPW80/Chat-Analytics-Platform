package ratelimit

import (
	"sync"
	"testing"
	"time"
)

func TestTokenBucket_AllowsBurst(t *testing.T) {
	b := NewTokenBucket(3, 1)
	frozen := time.Now()
	b.now = func() time.Time { return frozen }

	for i := 0; i < 3; i++ {
		if !b.Allow() {
			t.Fatalf("expected token %d within burst to be allowed", i)
		}
	}
	if b.Allow() {
		t.Error("expected 4th request to be denied once burst is exhausted")
	}
}

func TestTokenBucket_Refills(t *testing.T) {
	b := NewTokenBucket(2, 2) // 2 tokens/sec
	now := time.Now()
	b.now = func() time.Time { return now }

	// Evaluate into variables so both calls run (|| would short-circuit).
	first, second := b.Allow(), b.Allow()
	if !first || !second {
		t.Fatal("expected initial burst of 2 to be allowed")
	}
	if b.Allow() {
		t.Fatal("expected bucket to be empty")
	}

	// Advance 1 second: 2 tokens/sec should restore 2 tokens.
	now = now.Add(time.Second)
	third, fourth := b.Allow(), b.Allow()
	if !third || !fourth {
		t.Error("expected refilled tokens after 1 second")
	}
	if b.Allow() {
		t.Error("expected bucket empty again after consuming refill")
	}
}

func TestTokenBucket_CapsAtCapacity(t *testing.T) {
	b := NewTokenBucket(2, 100)
	now := time.Now()
	b.now = func() time.Time { return now }

	// A long idle period must not let tokens exceed capacity.
	now = now.Add(time.Hour)
	allowed := 0
	for i := 0; i < 10; i++ {
		if b.Allow() {
			allowed++
		}
	}
	if allowed != 2 {
		t.Errorf("expected tokens capped at capacity 2, got %d", allowed)
	}
}

func TestTokenBucket_ConcurrentSafe(t *testing.T) {
	b := NewTokenBucket(100, 100)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Allow()
		}()
	}
	wg.Wait()
}
