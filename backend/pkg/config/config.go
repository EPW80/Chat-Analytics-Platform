package config

import (
	"os"
)

// Config holds all configuration for the application
type Config struct {
	Port             string
	DynamoDBEndpoint string
	DynamoDBRegion   string
	AWSAccessKey     string
	AWSSecretKey     string
	LogLevel         string
}

// Load reads configuration from environment variables
func Load() *Config {
	return &Config{
		Port:             getEnv("PORT", "8080"),
		DynamoDBEndpoint: getEnv("DYNAMODB_ENDPOINT", "http://localhost:8000"),
		DynamoDBRegion:   getEnv("DYNAMODB_REGION", "us-east-1"),
		AWSAccessKey:     getEnv("AWS_ACCESS_KEY_ID", "dummy"),
		AWSSecretKey:     getEnv("AWS_SECRET_ACCESS_KEY", "dummy"),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
	}
}

// getEnv reads an environment variable with a fallback default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
