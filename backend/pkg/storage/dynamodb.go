package storage

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	appconfig "github.com/epw80/chat-analytics-platform/pkg/config"
	"github.com/epw80/chat-analytics-platform/pkg/message"
	"github.com/google/uuid"
)

// DynamoDBRepository implements MessageRepository using AWS DynamoDB
type DynamoDBRepository struct {
	client *dynamodb.Client
	logger *slog.Logger
}

// NewDynamoDBRepository creates a new DynamoDB-backed message repository
func NewDynamoDBRepository(ctx context.Context, cfg *appconfig.Config, logger *slog.Logger) (*DynamoDBRepository, error) {
	// Load AWS config
	var awsCfg aws.Config
	var err error

	// If using local DynamoDB endpoint, configure with static credentials
	if cfg.DynamoDBEndpoint != "" {
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.DynamoDBRegion),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.AWSAccessKey,
				cfg.AWSSecretKey,
				"",
			)),
		)
	} else {
		// Use default AWS credentials chain for production
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.DynamoDBRegion),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create DynamoDB client
	client := dynamodb.NewFromConfig(awsCfg, func(o *dynamodb.Options) {
		if cfg.DynamoDBEndpoint != "" {
			o.BaseEndpoint = aws.String(cfg.DynamoDBEndpoint)
		}
	})

	repo := &DynamoDBRepository{
		client: client,
		logger: logger,
	}

	// Verify connection with health check
	if err := repo.HealthCheck(ctx); err != nil {
		return nil, fmt.Errorf("DynamoDB health check failed: %w", err)
	}

	logger.Info("DynamoDB repository initialized",
		slog.String("region", cfg.DynamoDBRegion),
		slog.String("endpoint", cfg.DynamoDBEndpoint))

	return repo, nil
}

// SaveMessage persists a message to DynamoDB
func (r *DynamoDBRepository) SaveMessage(ctx context.Context, msg *message.Message) error {
	// Generate UUID if not already set
	if msg.MessageID == "" {
		msg.MessageID = uuid.New().String()
	}

	// Set default room if not specified
	if msg.RoomID == "" {
		msg.RoomID = DefaultRoomID
	}

	// Marshal message to DynamoDB attribute values
	item, err := attributevalue.MarshalMap(msg)
	if err != nil {
		r.logger.Error("failed to marshal message",
			slog.String("error", err.Error()),
			slog.String("messageId", msg.MessageID))
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Put item to DynamoDB
	input := &dynamodb.PutItemInput{
		TableName: aws.String(TableName),
		Item:      item,
	}

	_, err = r.client.PutItem(ctx, input)
	if err != nil {
		r.logger.Error("failed to save message to DynamoDB",
			slog.String("error", err.Error()),
			slog.String("messageId", msg.MessageID))
		return fmt.Errorf("failed to save message: %w", err)
	}

	r.logger.Debug("message saved to DynamoDB",
		slog.String("messageId", msg.MessageID),
		slog.String("roomId", msg.RoomID),
		slog.String("userId", msg.UserID))

	return nil
}

// GetRecentMessages retrieves the most recent messages for a given room
func (r *DynamoDBRepository) GetRecentMessages(ctx context.Context, roomID string, limit int) ([]*message.Message, error) {
	if roomID == "" {
		roomID = DefaultRoomID
	}

	// Query using GSI2 (RoomID + Timestamp) for chronological order
	input := &dynamodb.QueryInput{
		TableName:              aws.String(TableName),
		IndexName:              aws.String(IndexRoomTimestamp),
		KeyConditionExpression: aws.String("#roomId = :roomId"),
		ExpressionAttributeNames: map[string]string{
			"#roomId": AttrRoomID,
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":roomId": &types.AttributeValueMemberS{Value: roomID},
		},
		ScanIndexForward: aws.Bool(false), // Descending order (newest first)
		Limit:            aws.Int32(int32(limit)),
	}

	result, err := r.client.Query(ctx, input)
	if err != nil {
		r.logger.Error("failed to query messages",
			slog.String("error", err.Error()),
			slog.String("roomId", roomID))
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}

	// Unmarshal results
	messages := make([]*message.Message, 0, len(result.Items))
	for _, item := range result.Items {
		var msg message.Message
		if err := attributevalue.UnmarshalMap(item, &msg); err != nil {
			r.logger.Error("failed to unmarshal message",
				slog.String("error", err.Error()))
			continue
		}
		messages = append(messages, &msg)
	}

	// Reverse to get chronological order (oldest first)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	r.logger.Debug("retrieved messages from DynamoDB",
		slog.String("roomId", roomID),
		slog.Int("count", len(messages)))

	return messages, nil
}

// GetMessagesByUser retrieves all messages sent by a specific user
func (r *DynamoDBRepository) GetMessagesByUser(ctx context.Context, userID string, limit int) ([]*message.Message, error) {
	// Query using GSI1 (UserID + Timestamp)
	input := &dynamodb.QueryInput{
		TableName:              aws.String(TableName),
		IndexName:              aws.String(IndexUserTimestamp),
		KeyConditionExpression: aws.String("#userId = :userId"),
		ExpressionAttributeNames: map[string]string{
			"#userId": AttrUserID,
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":userId": &types.AttributeValueMemberS{Value: userID},
		},
		ScanIndexForward: aws.Bool(true), // Ascending order (oldest first)
		Limit:            aws.Int32(int32(limit)),
	}

	result, err := r.client.Query(ctx, input)
	if err != nil {
		r.logger.Error("failed to query user messages",
			slog.String("error", err.Error()),
			slog.String("userId", userID))
		return nil, fmt.Errorf("failed to query user messages: %w", err)
	}

	// Unmarshal results
	messages := make([]*message.Message, 0, len(result.Items))
	for _, item := range result.Items {
		var msg message.Message
		if err := attributevalue.UnmarshalMap(item, &msg); err != nil {
			r.logger.Error("failed to unmarshal message",
				slog.String("error", err.Error()))
			continue
		}
		messages = append(messages, &msg)
	}

	r.logger.Debug("retrieved user messages from DynamoDB",
		slog.String("userId", userID),
		slog.Int("count", len(messages)))

	return messages, nil
}

// HealthCheck verifies DynamoDB is accessible
func (r *DynamoDBRepository) HealthCheck(ctx context.Context) error {
	// Try to describe the table
	input := &dynamodb.DescribeTableInput{
		TableName: aws.String(TableName),
	}

	_, err := r.client.DescribeTable(ctx, input)
	if err != nil {
		return fmt.Errorf("DynamoDB health check failed: %w", err)
	}

	return nil
}

// Close releases resources (DynamoDB client doesn't need explicit cleanup)
func (r *DynamoDBRepository) Close() error {
	r.logger.Info("DynamoDB repository closed")
	return nil
}
