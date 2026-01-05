package storage

import (
	"testing"
)

// Unit tests for storage package
// Note: These are basic placeholder tests. Full unit tests would require
// mocking the DynamoDB client, which is complex with AWS SDK v2.
// Integration tests (integration_test.go) will provide comprehensive coverage
// with actual DynamoDB Local.

func TestGetTableSchema(t *testing.T) {
	schema := GetTableSchema()

	tests := []struct {
		name     string
		expected string
		actual   string
	}{
		{"TableName", TableName, schema.TableName},
		{"PartitionKey", AttrRoomID, schema.PartitionKey},
		{"SortKey", AttrMessageID, schema.SortKey},
		{"GSI1 Name", IndexUserTimestamp, schema.GSI1Name},
		{"GSI1 PartitionKey", AttrUserID, schema.GSI1PartitionKey},
		{"GSI1 SortKey", AttrTimestamp, schema.GSI1SortKey},
		{"GSI2 Name", IndexRoomTimestamp, schema.GSI2Name},
		{"GSI2 PartitionKey", AttrRoomID, schema.GSI2PartitionKey},
		{"GSI2 SortKey", AttrTimestamp, schema.GSI2SortKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.actual != tt.expected {
				t.Errorf("%s mismatch: expected %s, got %s", tt.name, tt.expected, tt.actual)
			}
		})
	}
}

func TestSchemaConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
	}{
		{"TableName", TableName},
		{"AttrMessageID", AttrMessageID},
		{"AttrRoomID", AttrRoomID},
		{"AttrType", AttrType},
		{"AttrUserID", AttrUserID},
		{"AttrUsername", AttrUsername},
		{"AttrContent", AttrContent},
		{"AttrTimestamp", AttrTimestamp},
		{"IndexUserTimestamp", IndexUserTimestamp},
		{"IndexRoomTimestamp", IndexRoomTimestamp},
		{"DefaultRoomID", DefaultRoomID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant == "" {
				t.Errorf("%s constant should not be empty", tt.name)
			}
		})
	}
}

func TestDefaultRoomID(t *testing.T) {
	if DefaultRoomID != "global" {
		t.Errorf("DefaultRoomID should be 'global', got '%s'", DefaultRoomID)
	}
}
