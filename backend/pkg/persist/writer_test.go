package persist

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/epw80/chat-analytics-platform/pkg/message"
)

type mockRepo struct {
	mu      sync.Mutex
	saved   []*message.Message
	batches int
	err     error
}

func (m *mockRepo) BatchSaveMessages(ctx context.Context, msgs []*message.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.batches++
	m.saved = append(m.saved, msgs...)
	return nil
}

func (m *mockRepo) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.saved)
}

func (m *mockRepo) batchCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.batches
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestWriter_PersistsAllOnClose(t *testing.T) {
	repo := &mockRepo{}
	w := New(repo, testLogger(), Config{Workers: 2, BatchSize: 10, FlushInterval: time.Hour})
	w.Start()

	for i := 0; i < 25; i++ {
		w.Enqueue(&message.Message{MessageID: string(rune('a' + i%26))})
	}
	w.Close() // must drain everything

	if got := repo.count(); got != 25 {
		t.Errorf("expected 25 messages persisted, got %d", got)
	}
}

func TestWriter_BatchesBySize(t *testing.T) {
	repo := &mockRepo{}
	// Single worker, batch of 5, no time-based flush during the test.
	w := New(repo, testLogger(), Config{Workers: 1, BatchSize: 5, FlushInterval: time.Hour})
	w.Start()

	for i := 0; i < 10; i++ {
		w.Enqueue(&message.Message{MessageID: "m"})
	}
	w.Close()

	if got := repo.count(); got != 10 {
		t.Fatalf("expected 10 messages, got %d", got)
	}
	// 10 messages / batch size 5 => at least 2 size-triggered batches.
	if got := repo.batchCount(); got < 2 {
		t.Errorf("expected >=2 batches, got %d", got)
	}
}

func TestWriter_FlushesOnInterval(t *testing.T) {
	repo := &mockRepo{}
	w := New(repo, testLogger(), Config{Workers: 1, BatchSize: 100, FlushInterval: 20 * time.Millisecond})
	w.Start()
	defer w.Close()

	w.Enqueue(&message.Message{MessageID: "m"})

	// Batch size is far from reached; only the interval can flush it.
	deadline := time.Now().Add(time.Second)
	for repo.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if repo.count() != 1 {
		t.Errorf("expected interval flush to persist 1 message, got %d", repo.count())
	}
}

func TestWriter_DropsWhenQueueFull(t *testing.T) {
	// Block the repo so the single worker can't drain, forcing queue overflow.
	release := make(chan struct{})
	repo := &blockingRepo{release: release}
	w := New(repo, testLogger(), Config{Workers: 1, BatchSize: 1, QueueSize: 1, FlushInterval: time.Hour})
	w.Start()

	// Flood well past worker(1)+queue(1) capacity.
	for i := 0; i < 100; i++ {
		w.Enqueue(&message.Message{MessageID: "m"})
	}
	if w.Dropped() == 0 {
		t.Error("expected some messages to be dropped when the queue is saturated")
	}

	close(release)
	w.Close()
}

func TestWriter_SurvivesRepoError(t *testing.T) {
	repo := &mockRepo{err: errors.New("boom")}
	w := New(repo, testLogger(), Config{Workers: 1, BatchSize: 1, FlushInterval: time.Hour})
	w.Start()
	w.Enqueue(&message.Message{MessageID: "m"})
	w.Close() // should not panic despite the repo error
}

func TestWriter_EnqueueAfterCloseIsSafe(t *testing.T) {
	repo := &mockRepo{}
	w := New(repo, testLogger(), Config{Workers: 1})
	w.Start()
	w.Close()
	// Must not panic on send to a closed queue.
	w.Enqueue(&message.Message{MessageID: "m"})
}

// blockingRepo blocks every batch until release is closed.
type blockingRepo struct {
	release chan struct{}
}

func (b *blockingRepo) BatchSaveMessages(ctx context.Context, msgs []*message.Message) error {
	<-b.release
	return nil
}
