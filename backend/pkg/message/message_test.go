package message

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMessage_Validate(t *testing.T) {
	tests := []struct {
		name    string
		msg     Message
		wantErr error
	}{
		{
			name: "valid chat message",
			msg: Message{
				Type:      TypeChat,
				UserID:    "user123",
				Username:  "alice",
				Content:   "Hello world",
				Timestamp: time.Now(),
			},
			wantErr: nil,
		},
		{
			name: "empty content for chat",
			msg: Message{
				Type:     TypeChat,
				Username: "alice",
				Content:  "",
			},
			wantErr: ErrEmptyContent,
		},
		{
			name: "content too long",
			msg: Message{
				Type:     TypeChat,
				Username: "alice",
				Content:  strings.Repeat("a", MaxContentLength+1),
			},
			wantErr: ErrContentTooLong,
		},
		{
			name: "empty username",
			msg: Message{
				Type:    TypeChat,
				Content: "hello",
			},
			wantErr: ErrEmptyUsername,
		},
		{
			name: "username too long",
			msg: Message{
				Type:     TypeChat,
				Username: strings.Repeat("a", MaxUsernameLength+1),
				Content:  "hello",
			},
			wantErr: ErrUsernameTooLong,
		},
		{
			name: "invalid type",
			msg: Message{
				Type:     Type("invalid"),
				Username: "alice",
				Content:  "hello",
			},
			wantErr: ErrInvalidType,
		},
		{
			name: "valid system message with empty content",
			msg: Message{
				Type:     TypeSystem,
				Username: "System",
				Content:  "",
			},
			wantErr: nil,
		},
		{
			name: "valid join message",
			msg: Message{
				Type:     TypeJoin,
				Username: "alice",
				Content:  "",
			},
			wantErr: nil,
		},
		{
			name: "valid leave message",
			msg: Message{
				Type:     TypeLeave,
				Username: "alice",
				Content:  "",
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.Validate()
			if err != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewChatMessage(t *testing.T) {
	msg := NewChatMessage("user123", "alice", "Hello world")

	if msg.Type != TypeChat {
		t.Errorf("expected type %v, got %v", TypeChat, msg.Type)
	}
	if msg.UserID != "user123" {
		t.Errorf("expected userID user123, got %v", msg.UserID)
	}
	if msg.Username != "alice" {
		t.Errorf("expected username alice, got %v", msg.Username)
	}
	if msg.Content != "Hello world" {
		t.Errorf("expected content 'Hello world', got %v", msg.Content)
	}
	if msg.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}
	if msg.Timestamp.Location() != time.UTC {
		t.Error("timestamp should be in UTC")
	}
}

func TestNewSystemMessage(t *testing.T) {
	msg := NewSystemMessage("Server announcement")

	if msg.Type != TypeSystem {
		t.Errorf("expected type %v, got %v", TypeSystem, msg.Type)
	}
	if msg.UserID != "system" {
		t.Errorf("expected userID system, got %v", msg.UserID)
	}
	if msg.Username != "System" {
		t.Errorf("expected username System, got %v", msg.Username)
	}
	if msg.Content != "Server announcement" {
		t.Errorf("expected content 'Server announcement', got %v", msg.Content)
	}
	if msg.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}
}

func TestMessage_JSON(t *testing.T) {
	original := NewChatMessage("user123", "alice", "Hello")

	// Marshal
	data, err := original.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	// Unmarshal
	parsed, err := FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON() error = %v", err)
	}

	// Compare
	if parsed.Type != original.Type {
		t.Errorf("type mismatch: got %v, want %v", parsed.Type, original.Type)
	}
	if parsed.UserID != original.UserID {
		t.Errorf("userID mismatch: got %v, want %v", parsed.UserID, original.UserID)
	}
	if parsed.Username != original.Username {
		t.Errorf("username mismatch: got %v, want %v", parsed.Username, original.Username)
	}
	if parsed.Content != original.Content {
		t.Errorf("content mismatch: got %v, want %v", parsed.Content, original.Content)
	}
}

func TestFromJSON_Invalid(t *testing.T) {
	_, err := FromJSON([]byte("invalid json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestMessage_JSONFields(t *testing.T) {
	msg := NewChatMessage("user123", "alice", "test")
	data, _ := msg.ToJSON()

	var result map[string]interface{}
	json.Unmarshal(data, &result)

	// Check JSON field names (camelCase)
	if _, ok := result["userId"]; !ok {
		t.Error("expected userId field in JSON")
	}
	if _, ok := result["username"]; !ok {
		t.Error("expected username field in JSON")
	}
	if _, ok := result["content"]; !ok {
		t.Error("expected content field in JSON")
	}
	if _, ok := result["timestamp"]; !ok {
		t.Error("expected timestamp field in JSON")
	}
	if _, ok := result["type"]; !ok {
		t.Error("expected type field in JSON")
	}
}

// Benchmark JSON operations
func BenchmarkMessage_ToJSON(b *testing.B) {
	msg := NewChatMessage("user123", "alice", "Hello world")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = msg.ToJSON()
	}
}

func BenchmarkFromJSON(b *testing.B) {
	msg := NewChatMessage("user123", "alice", "Hello world")
	data, _ := json.Marshal(msg)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = FromJSON(data)
	}
}

func BenchmarkMessage_Validate(b *testing.B) {
	msg := NewChatMessage("user123", "alice", "Hello world")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = msg.Validate()
	}
}
