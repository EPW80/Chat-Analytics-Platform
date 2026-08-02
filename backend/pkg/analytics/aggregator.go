package analytics

import (
	"sync"
	"sync/atomic"
	"time"
)

// slidingWindow tracks a per-minute message count over the last 15 minutes.
// Each slot represents one minute; the window rotates on minute boundaries.
type slidingWindow struct {
	mu       sync.Mutex
	slots    [15]atomic.Int64
	idx      int
	lastTick time.Time
}

func newWindow() *slidingWindow {
	return &slidingWindow{lastTick: time.Now().Truncate(time.Minute)}
}

// increment counts one message into the current minute. The lock is held across
// both the rotation check and the add: releasing it in between lets a concurrent
// rotation advance idx and zero the slot after this call has chosen its index,
// dropping the count into a slot that is immediately cleared.
func (w *slidingWindow) increment() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.advanceIfNeeded()
	w.slots[w.idx].Add(1)
}

// snapshot returns the counts for the last 15 minutes, oldest first.
func (w *slidingWindow) snapshot() []int64 {
	w.mu.Lock()
	w.advanceIfNeeded()
	start := w.idx + 1 // oldest slot is one past current
	w.mu.Unlock()

	out := make([]int64, 15)
	for i := range out {
		out[i] = w.slots[(start+i)%15].Load()
	}
	return out
}

// advanceIfNeeded rotates the window forward, zeroing expired slots.
// Must be called with w.mu held.
func (w *slidingWindow) advanceIfNeeded() {
	now := time.Now().Truncate(time.Minute)
	if !now.After(w.lastTick) {
		return
	}
	minutes := int(now.Sub(w.lastTick).Minutes())
	if minutes > 15 {
		minutes = 15
	}
	for i := 0; i < minutes; i++ {
		w.idx = (w.idx + 1) % 15
		w.slots[w.idx].Store(0)
	}
	w.lastTick = now
}
