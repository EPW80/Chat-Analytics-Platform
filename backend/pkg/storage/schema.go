package storage

const (
	// TableName is the name of the DynamoDB table for storing chat messages
	TableName = "chat-messages"

	// Attribute names
	AttrMessageID = "MessageID"
	AttrRoomID    = "RoomID"
	AttrType      = "Type"
	AttrUserID    = "UserID"
	AttrUsername  = "Username"
	AttrContent   = "Content"
	AttrTimestamp = "Timestamp"

	// Index names
	IndexUserTimestamp = "UserID-Timestamp-index"
	IndexRoomTimestamp = "RoomID-Timestamp-index"

	// Default room ID (for single-room chat)
	DefaultRoomID = "global"
)

// TableSchema returns the DynamoDB table creation parameters
type TableSchema struct {
	TableName string
	// Primary key
	PartitionKey string
	SortKey      string
	// Global secondary indexes
	GSI1PartitionKey string
	GSI1SortKey      string
	GSI1Name         string
	GSI2PartitionKey string
	GSI2SortKey      string
	GSI2Name         string
}

// GetTableSchema returns the schema configuration for the messages table
func GetTableSchema() TableSchema {
	return TableSchema{
		TableName:        TableName,
		PartitionKey:     AttrRoomID,
		SortKey:          AttrMessageID,
		GSI1PartitionKey: AttrUserID,
		GSI1SortKey:      AttrTimestamp,
		GSI1Name:         IndexUserTimestamp,
		GSI2PartitionKey: AttrRoomID,
		GSI2SortKey:      AttrTimestamp,
		GSI2Name:         IndexRoomTimestamp,
	}
}
