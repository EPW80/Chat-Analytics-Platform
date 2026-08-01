package briefing

import "sync"

// Store is a bounded, concurrency-safe ring buffer of recent briefings. It
// deliberately mirrors the in-memory latencySamples pattern in analytics/tracker.go
// (RWMutex, fixed cap, evict-oldest) rather than persisting to DynamoDB, per the
// repo convention that reserves schema work for hand-writing. DynamoDB persistence
// is a documented stretch, not v1.
type Store struct {
	mu  sync.RWMutex
	cap int
	buf []Record // oldest first; len grows to cap then evicts head
}

// NewStore returns a Store holding at most `capacity` recent records.
func NewStore(capacity int) *Store {
	if capacity <= 0 {
		capacity = 50
	}
	return &Store{cap: capacity, buf: make([]Record, 0, capacity)}
}

// Add appends a record, evicting the oldest when at capacity.
func (s *Store) Add(r Record) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.buf) >= s.cap {
		// Shift rather than reslice: reslicing off the head leaks the evicted
		// records' backing array for the life of the process.
		copy(s.buf, s.buf[len(s.buf)-s.cap+1:])
		s.buf = s.buf[:s.cap-1]
	}
	s.buf = append(s.buf, r)
}

// Recent returns up to n newest-first records for GET /api/briefings. n <= 0
// returns everything held.
func (s *Store) Recent(n int) []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if n <= 0 || n > len(s.buf) {
		n = len(s.buf)
	}
	out := make([]Record, 0, n)
	for i := len(s.buf) - 1; i >= len(s.buf)-n; i-- {
		out = append(out, s.buf[i])
	}
	return out
}

// ByHash finds a record for GET /api/briefings/{hash}/audio. ok=false if absent.
// A linear scan is right at this size — the buffer holds tens of records, and a
// second index would have to be kept consistent through eviction.
func (s *Store) ByHash(hash string) (Record, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := len(s.buf) - 1; i >= 0; i-- {
		if s.buf[i].Hash == hash {
			return s.buf[i], true
		}
	}
	return Record{}, false
}

// Len reports how many records are held. Used by tests and diagnostics.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.buf)
}
