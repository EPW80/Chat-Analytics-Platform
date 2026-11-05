// Command init-dynamodb ensures the chat-messages table exists without
// destroying existing data. Prefer this over delete/recreate when bootstrapping
// a local environment; the server also calls EnsureTable on startup.
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	appconfig "github.com/epw80/chat-analytics-platform/pkg/config"
	"github.com/epw80/chat-analytics-platform/pkg/storage"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	cfg := appconfig.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	logger.Info("ensuring DynamoDB table exists",
		slog.String("endpoint", cfg.DynamoDBEndpoint),
		slog.String("region", cfg.DynamoDBRegion),
		slog.String("table", storage.TableName))

	repo, err := storage.NewDynamoDBRepository(ctx, cfg, logger)
	if err != nil {
		log.Fatalf("Failed to ensure DynamoDB table: %v", err)
	}
	defer func() {
		_ = repo.Close()
	}()

	if err := repo.HealthCheck(ctx); err != nil {
		log.Fatalf("Table health check failed: %v", err)
	}

	fmt.Printf("\nDynamoDB table ready: %s\n", storage.TableName)
}
