package briefing

import (
	"strconv"
	"sync"
	"testing"
)

func TestStore_EvictsOldest(t *testing.T) {
	s := NewStore(2)
	s.Add(Record{Hash: "a"})
	s.Add(Record{Hash: "b"})
	s.Add(Record{Hash: "c"}) // should evict "a"

	if _, ok := s.ByHash("a"); ok {
		t.Error("expected oldest record 'a' to be evicted")
	}
	if got := s.Recent(10); len(got) != 2 {
		t.Fatalf("expected 2 records, got %d", len(got))
	}
	for _, hash := range []string{"b", "c"} {
		if _, ok := s.ByHash(hash); !ok {
			t.Errorf("expected %q to still be held", hash)
		}
	}
}

func TestStore_RecentIsNewestFirst(t *testing.T) {
	s := NewStore(10)
	for _, hash := range []string{"a", "b", "c"} {
		s.Add(Record{Hash: hash})
	}

	got := s.Recent(0) // 0 means "everything"
	want := []string{"c", "b", "a"}
	if len(got) != len(want) {
		t.Fatalf("got %d records, want %d", len(got), len(want))
	}
	for i, hash := range want {
		if got[i].Hash != hash {
			t.Errorf("Recent()[%d] = %q, want %q", i, got[i].Hash, hash)
		}
	}

	if limited := s.Recent(2); len(limited) != 2 || limited[0].Hash != "c" {
		t.Errorf("Recent(2) = %v, want the two newest starting at 'c'", limited)
	}
}

func TestStore_EmptyAndDefaults(t *testing.T) {
	s := NewStore(0) // non-positive capacity falls back to the default
	if got := s.Recent(5); len(got) != 0 {
		t.Errorf("empty store returned %d records", len(got))
	}
	if _, ok := s.ByHash("nope"); ok {
		t.Error("empty store claimed to hold a hash")
	}
	if s.Len() != 0 {
		t.Errorf("Len() = %d on an empty store", s.Len())
	}
}

// The store never grows past its cap, however long the server runs.
func TestStore_StaysBounded(t *testing.T) {
	s := NewStore(3)
	for i := 0; i < 100; i++ {
		s.Add(Record{Hash: strconv.Itoa(i)})
	}
	if s.Len() != 3 {
		t.Fatalf("Len() = %d, want the cap 3", s.Len())
	}
	if got := s.Recent(0); len(got) != 3 || got[0].Hash != "99" {
		t.Errorf("Recent() = %v, want the three newest starting at '99'", got)
	}
}

// Anomalies are handled on their own goroutines, so Add races Recent by design.
func TestStore_ConcurrentAccess(t *testing.T) {
	s := NewStore(20)
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			s.Add(Record{Hash: strconv.Itoa(n)})
		}(i)
		go func() {
			defer wg.Done()
			s.Recent(5)
			s.ByHash("7")
		}()
	}
	wg.Wait()

	if s.Len() != 20 {
		t.Errorf("Len() = %d, want the cap 20", s.Len())
	}
}
