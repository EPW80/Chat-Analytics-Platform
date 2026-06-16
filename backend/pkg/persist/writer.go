// Package persist provides a bounded, batching writer that decouples the hot
// WebSocket path from storage latency. Messages are enqueued without blocking
// and flushed to the repository by a fixed pool of workers.
package persist

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/epw80/chat-analytics-platform/pkg/message"
)

// Repository is the subset of storage the writer needs.
type Repository interface {
	BatchSaveMessages(ctx context.Context, msgs []*message.Message) error
}

// Config tunes the writer. Non-positive fields fall back to defaults.
type Config struct {
	Workers       int
	BatchSize     int
	QueueSize     int
	FlushInterval time.Duration
}

const (
	defaultWorkers       = 4
	defaultBatchSize     = 25 // DynamoDB BatchWriteItem hard limit
	defaultQueueSize     = 1024
	defaultFlushInterval = 500 * time.Millisecond
	maxBatchSize         = 25
	writeTimeout         = 5 * time.Second
)

// Writer batches enqueued messages and persists them via a bounded worker pool.
type Writer struct {
	repo          Repository
	logger        *slog.Logger
	queue         chan *message.Message
	batchSize     int
	flushInterval time.Duration
	workers       int

	wg      sync.WaitGroup
	mu      sync.RWMutex // guards closed, serialized against Enqueue sends
	closed  bool
	dropped atomic.Int64
}

// New builds a Writer. Call Start to launch the workers.
func New(repo Repository, logger *slog.Logger, cfg Config) *Writer {
	if cfg.Workers <= 0 {
		cfg.Workers = defaultWorkers
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaultBatchSize
	}
	if cfg.BatchSize > maxBatchSize {
		cfg.BatchSize = maxBatchSize
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = defaultQueueSize
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = defaultFlushInterval
	}
	return &Writer{
		repo:          repo,
		logger:        logger,
		queue:         make(chan *message.Message, cfg.QueueSize),
		batchSize:     cfg.BatchSize,
		flushInterval: cfg.FlushInterval,
		workers:       cfg.Workers,
	}
}

// Start launches the worker pool.
func (w *Writer) Start() {
	for i := 0; i < w.workers; i++ {
		w.wg.Add(1)
		go w.worker()
	}
}

// Enqueue submits a message for asynchronous persistence. It never blocks: if
// the queue is full the message is dropped and counted. Safe to call after
// Close (the message is silently dropped).
func (w *Writer) Enqueue(msg *message.Message) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.closed {
		return
	}
	select {
	case w.queue <- msg:
	default:
		n := w.dropped.Add(1)
		w.logger.Warn("persistence queue full, dropping message",
			slog.String("messageID", msg.MessageID),
			slog.Int64("totalDropped", n))
	}
}

// Dropped returns the number of messages dropped because the queue was full.
func (w *Writer) Dropped() int64 {
	return w.dropped.Load()
}

// Close stops accepting messages and waits for in-flight batches to flush.
func (w *Writer) Close() {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.closed = true
	close(w.queue)
	w.mu.Unlock()

	w.wg.Wait()
}

func (w *Writer) worker() {
	defer w.wg.Done()

	batch := make([]*message.Message, 0, w.batchSize)
	timer := time.NewTimer(w.flushInterval)
	defer timer.Stop()

	resetTimer := func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(w.flushInterval)
	}

	flush := func() {
		if len(batch) == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
		if err := w.repo.BatchSaveMessages(ctx, batch); err != nil {
			w.logger.Error("failed to persist message batch",
				slog.Int("batchSize", len(batch)),
				slog.String("error", err.Error()))
		}
		cancel()
		batch = batch[:0]
	}

	for {
		select {
		case msg, ok := <-w.queue:
			if !ok {
				flush() // drain remaining on shutdown
				return
			}
			batch = append(batch, msg)
			if len(batch) >= w.batchSize {
				flush()
				resetTimer()
			}
		case <-timer.C:
			flush()
			timer.Reset(w.flushInterval)
		}
	}
}
