package message

import (
	"encoding/json"
	"errors"
	"time"
)

// Type represents different message types in the system
type Type string

const (
	TypeChat   Type = "chat"
	TypeSystem Type = "system"
	TypeJoin   Type = "join"
	TypeLeave  Type = "leave"
)

// Message represents a WebSocket message
type Message struct {
	MessageID string    `json:"messageId" dynamodbav:"MessageID"`
	RoomID    string    `json:"roomId" dynamodbav:"RoomID"`
	Type      Type      `json:"type" dynamodbav:"Type"`
	UserID    string    `json:"userId" dynamodbav:"UserID"`
	Username  string    `json:"username" dynamodbav:"Username"`
	Content   string    `json:"content" dynamodbav:"Content"`
	Timestamp time.Time `json:"timestamp" dynamodbav:"Timestamp"`
}

// Validation constants
const (
	MaxContentLength  = 1000
	MaxUsernameLength = 50
)

var (
	ErrEmptyContent    = errors.New("message content cannot be empty")
	ErrContentTooLong  = errors.New("message content exceeds maximum length")
	ErrEmptyUsername   = errors.New("username cannot be empty")
	ErrUsernameTooLong = errors.New("username exceeds maximum length")
	ErrInvalidType     = errors.New("invalid message type")
)

// Validate checks if the message meets all requirements
func (m *Message) Validate() error {
	// Type validation
	if m.Type != TypeChat && m.Type != TypeSystem && m.Type != TypeJoin && m.Type != TypeLeave {
		return ErrInvalidType
	}

	// Username validation
	if m.Username == "" {
		return ErrEmptyUsername
	}
	if len(m.Username) > MaxUsernameLength {
		return ErrUsernameTooLong
	}

	// Content validation (only for chat messages)
	if m.Type == TypeChat {
		if m.Content == "" {
			return ErrEmptyContent
		}
		if len(m.Content) > MaxContentLength {
			return ErrContentTooLong
		}
	}

	return nil
}

// NewChatMessage creates a new chat message
func NewChatMessage(userID, username, content string) *Message {
	return &Message{
		Type:      TypeChat,
		UserID:    userID,
		Username:  username,
		Content:   content,
		Timestamp: time.Now().UTC(),
	}
}

// NewSystemMessage creates a new system message
func NewSystemMessage(content string) *Message {
	return &Message{
		Type:      TypeSystem,
		UserID:    "system",
		Username:  "System",
		Content:   content,
		Timestamp: time.Now().UTC(),
	}
}

// ToJSON converts message to JSON bytes
func (m *Message) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// FromJSON parses JSON bytes into a message
func FromJSON(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}
