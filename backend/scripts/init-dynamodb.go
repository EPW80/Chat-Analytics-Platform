package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	appconfig "github.com/epw80/chat-analytics-platform/pkg/config"
	"github.com/epw80/chat-analytics-platform/pkg/storage"
)

func main() {
	ctx := context.Background()

	// Load configuration
	cfg := appconfig.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	logger.Info("Initializing DynamoDB tables",
		slog.String("endpoint", cfg.DynamoDBEndpoint),
		slog.String("region", cfg.DynamoDBRegion))

	// Configure AWS SDK
	var awsCfg aws.Config
	var err error

	if cfg.DynamoDBEndpoint != "" {
		// Local DynamoDB
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.DynamoDBRegion),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.AWSAccessKey,
				cfg.AWSSecretKey,
				"",
			)),
		)
	} else {
		// AWS DynamoDB
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.DynamoDBRegion),
		)
	}

	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	// Create DynamoDB client
	client := dynamodb.NewFromConfig(awsCfg, func(o *dynamodb.Options) {
		if cfg.DynamoDBEndpoint != "" {
			o.BaseEndpoint = aws.String(cfg.DynamoDBEndpoint)
		}
	})

	// Get table schema
	schema := storage.GetTableSchema()

	// Check if table already exists
	_, err = client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(schema.TableName),
	})

	if err == nil {
		logger.Info("Table already exists, deleting and recreating",
			slog.String("table", schema.TableName))

		// Delete existing table
		_, err = client.DeleteTable(ctx, &dynamodb.DeleteTableInput{
			TableName: aws.String(schema.TableName),
		})
		if err != nil {
			log.Fatalf("Failed to delete existing table: %v", err)
		}

		// Wait for table to be deleted
		waiter := dynamodb.NewTableNotExistsWaiter(client)
		err = waiter.Wait(ctx, &dynamodb.DescribeTableInput{
			TableName: aws.String(schema.TableName),
		}, 60)
		if err != nil {
			log.Fatalf("Failed waiting for table deletion: %v", err)
		}

		logger.Info("Existing table deleted successfully")
	}

	// Create table
	logger.Info("Creating DynamoDB table",
		slog.String("table", schema.TableName))

	createTableInput := &dynamodb.CreateTableInput{
		TableName: aws.String(schema.TableName),
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String(schema.PartitionKey),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String(schema.SortKey),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String(schema.GSI1PartitionKey),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String(schema.GSI1SortKey),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String(schema.PartitionKey),
				KeyType:       types.KeyTypeHash,
			},
			{
				AttributeName: aws.String(schema.SortKey),
				KeyType:       types.KeyTypeRange,
			},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			{
				IndexName: aws.String(schema.GSI1Name),
				KeySchema: []types.KeySchemaElement{
					{
						AttributeName: aws.String(schema.GSI1PartitionKey),
						KeyType:       types.KeyTypeHash,
					},
					{
						AttributeName: aws.String(schema.GSI1SortKey),
						KeyType:       types.KeyTypeRange,
					},
				},
				Projection: &types.Projection{
					ProjectionType: types.ProjectionTypeAll,
				},
				ProvisionedThroughput: &types.ProvisionedThroughput{
					ReadCapacityUnits:  aws.Int64(5),
					WriteCapacityUnits: aws.Int64(5),
				},
			},
			{
				IndexName: aws.String(schema.GSI2Name),
				KeySchema: []types.KeySchemaElement{
					{
						AttributeName: aws.String(schema.GSI2PartitionKey),
						KeyType:       types.KeyTypeHash,
					},
					{
						AttributeName: aws.String(schema.GSI2SortKey),
						KeyType:       types.KeyTypeRange,
					},
				},
				Projection: &types.Projection{
					ProjectionType: types.ProjectionTypeAll,
				},
				ProvisionedThroughput: &types.ProvisionedThroughput{
					ReadCapacityUnits:  aws.Int64(5),
					WriteCapacityUnits: aws.Int64(5),
				},
			},
		},
		BillingMode: types.BillingModeProvisioned,
		ProvisionedThroughput: &types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		},
	}

	_, err = client.CreateTable(ctx, createTableInput)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	// Wait for table to be active
	waiter := dynamodb.NewTableExistsWaiter(client)
	err = waiter.Wait(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(schema.TableName),
	}, 60)
	if err != nil {
		log.Fatalf("Failed waiting for table creation: %v", err)
	}

	logger.Info("Table created successfully",
		slog.String("table", schema.TableName))

	// Describe table to verify
	output, err := client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(schema.TableName),
	})
	if err != nil {
		log.Fatalf("Failed to describe table: %v", err)
	}

	fmt.Println("\n✅ DynamoDB Table Created Successfully!")
	fmt.Printf("\nTable: %s\n", *output.Table.TableName)
	fmt.Printf("Status: %s\n", output.Table.TableStatus)
	fmt.Printf("Item Count: %d\n", output.Table.ItemCount)
	fmt.Printf("\nPrimary Key:\n")
	fmt.Printf("  - Partition Key: %s (HASH)\n", schema.PartitionKey)
	fmt.Printf("  - Sort Key: %s (RANGE)\n", schema.SortKey)
	fmt.Printf("\nGlobal Secondary Indexes:\n")
	fmt.Printf("  1. %s\n", schema.GSI1Name)
	fmt.Printf("     - Partition Key: %s\n", schema.GSI1PartitionKey)
	fmt.Printf("     - Sort Key: %s\n", schema.GSI1SortKey)
	fmt.Printf("  2. %s\n", schema.GSI2Name)
	fmt.Printf("     - Partition Key: %s\n", schema.GSI2PartitionKey)
	fmt.Printf("     - Sort Key: %s\n", schema.GSI2SortKey)
	fmt.Println("\n✅ Ready to use!")
}
