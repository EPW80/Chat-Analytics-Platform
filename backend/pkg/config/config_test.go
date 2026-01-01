package config

import (
	"os"
	"testing"
)

func TestLoad_DefaultValues(t *testing.T) {
	// Clear environment variables
	os.Unsetenv("PORT")
	os.Unsetenv("DYNAMODB_ENDPOINT")
	os.Unsetenv("DYNAMODB_REGION")
	os.Unsetenv("AWS_ACCESS_KEY_ID")
	os.Unsetenv("AWS_SECRET_ACCESS_KEY")
	os.Unsetenv("LOG_LEVEL")

	cfg := Load()

	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"Port", cfg.Port, "8080"},
		{"DynamoDBEndpoint", cfg.DynamoDBEndpoint, "http://localhost:8000"},
		{"DynamoDBRegion", cfg.DynamoDBRegion, "us-east-1"},
		{"AWSAccessKey", cfg.AWSAccessKey, "dummy"},
		{"AWSSecretKey", cfg.AWSSecretKey, "dummy"},
		{"LogLevel", cfg.LogLevel, "info"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.expected)
			}
		})
	}
}

func TestLoad_EnvironmentVariables(t *testing.T) {
	// Set custom environment variables
	os.Setenv("PORT", "9000")
	os.Setenv("DYNAMODB_ENDPOINT", "http://dynamodb:8000")
	os.Setenv("DYNAMODB_REGION", "us-west-2")
	os.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
	os.Setenv("LOG_LEVEL", "debug")

	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("DYNAMODB_ENDPOINT")
		os.Unsetenv("DYNAMODB_REGION")
		os.Unsetenv("AWS_ACCESS_KEY_ID")
		os.Unsetenv("AWS_SECRET_ACCESS_KEY")
		os.Unsetenv("LOG_LEVEL")
	}()

	cfg := Load()

	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"Port", cfg.Port, "9000"},
		{"DynamoDBEndpoint", cfg.DynamoDBEndpoint, "http://dynamodb:8000"},
		{"DynamoDBRegion", cfg.DynamoDBRegion, "us-west-2"},
		{"AWSAccessKey", cfg.AWSAccessKey, "test-key"},
		{"AWSSecretKey", cfg.AWSSecretKey, "test-secret"},
		{"LogLevel", cfg.LogLevel, "debug"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.expected)
			}
		})
	}
}

func TestLoad_PartialEnvironment(t *testing.T) {
	// Set only some environment variables
	os.Setenv("PORT", "3000")
	os.Setenv("LOG_LEVEL", "error")

	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("LOG_LEVEL")
	}()

	cfg := Load()

	// Custom values
	if cfg.Port != "3000" {
		t.Errorf("Port = %q, want %q", cfg.Port, "3000")
	}
	if cfg.LogLevel != "error" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "error")
	}

	// Default values
	if cfg.DynamoDBEndpoint != "http://localhost:8000" {
		t.Errorf("DynamoDBEndpoint = %q, want default", cfg.DynamoDBEndpoint)
	}
	if cfg.DynamoDBRegion != "us-east-1" {
		t.Errorf("DynamoDBRegion = %q, want default", cfg.DynamoDBRegion)
	}
}

func TestGetEnv_WithValue(t *testing.T) {
	os.Setenv("TEST_VAR", "test-value")
	defer os.Unsetenv("TEST_VAR")

	result := getEnv("TEST_VAR", "default")
	if result != "test-value" {
		t.Errorf("getEnv() = %q, want %q", result, "test-value")
	}
}

func TestGetEnv_WithoutValue(t *testing.T) {
	os.Unsetenv("MISSING_VAR")

	result := getEnv("MISSING_VAR", "default-value")
	if result != "default-value" {
		t.Errorf("getEnv() = %q, want %q", result, "default-value")
	}
}

func TestGetEnv_EmptyString(t *testing.T) {
	os.Setenv("EMPTY_VAR", "")
	defer os.Unsetenv("EMPTY_VAR")

	// Empty string should return default
	result := getEnv("EMPTY_VAR", "default")
	if result != "default" {
		t.Errorf("getEnv() with empty string = %q, want %q", result, "default")
	}
}
