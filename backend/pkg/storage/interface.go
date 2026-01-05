package storage

import (
	"context"

	"github.com/epw80/chat-analytics-platform/pkg/message"
)

// MessageRepository defines the interface for message persistence operations.
// Implementations should be safe for concurrent use.
type MessageRepository interface {
	// SaveMessage persists a message to storage.
	// Returns an error if the operation fails.
	SaveMessage(ctx context.Context, msg *message.Message) error

	// GetRecentMessages retrieves the most recent messages for a given room.
	// Messages are returned in chronological order (oldest first).
	// The limit parameter controls the maximum number of messages to return.
	GetRecentMessages(ctx context.Context, roomID string, limit int) ([]*message.Message, error)

	// GetMessagesByUser retrieves all messages sent by a specific user.
	// Messages are returned in chronological order (oldest first).
	// The limit parameter controls the maximum number of messages to return.
	GetMessagesByUser(ctx context.Context, userID string, limit int) ([]*message.Message, error)

	// HealthCheck verifies the storage backend is accessible and operational.
	// Returns an error if the storage backend is unavailable.
	HealthCheck(ctx context.Context) error

	// Close releases any resources held by the repository.
	// After calling Close, the repository should not be used.
	Close() error
}
