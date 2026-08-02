package storage

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Unit tests for storage package
// Note: These are basic placeholder tests. Full unit tests would require
// mocking the DynamoDB client, which is complex with AWS SDK v2.
// Integration tests (integration_test.go) will provide comprehensive coverage
// with actual DynamoDB Local.

// The distinction isTableNotFound draws is what keeps a throttle or a transport
// blip from being read as "the table is absent" and sent down the create path,
// where CreateTable answers ResourceInUseException and init fails outright —
// dropping the server into "running without persistence" while DynamoDB is fine.
//
// This is unit-tested directly rather than through EnsureTable because the
// package has no AWS client mock (see the note above); the waiter and the call
// path around it are covered by the DynamoDB Local run instead.
func TestIsTableNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "bare not-found",
			err:  &types.ResourceNotFoundException{},
			want: true,
		},
		{
			name: "wrapped not-found",
			err:  fmt.Errorf("describe table: %w", &types.ResourceNotFoundException{}),
			want: true,
		},
		{
			name: "throughput exceeded is not absence",
			err:  &types.ProvisionedThroughputExceededException{},
			want: false,
		},
		{
			name: "unrelated error is not absence",
			err:  errors.New("dial tcp: connection refused"),
			want: false,
		},
		{
			name: "nil is not absence",
			err:  nil,
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isTableNotFound(tc.err); got != tc.want {
				t.Errorf("isTableNotFound(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

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
