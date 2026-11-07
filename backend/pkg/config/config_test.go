package config

import (
	"testing"
	"time"
)

func TestLoad_DefaultValues(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("DYNAMODB_ENDPOINT", "")
	t.Setenv("DYNAMODB_REGION", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("PERSIST_FLUSH_INTERVAL", "")

	// Empty string falls back via getEnv; clear by unsetting through empty
	// isn't enough for all keys if the process inherited values, so use
	// explicit blanks only where getEnv treats "" as missing.
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

	if cfg.PersistFlushInterval != 500*time.Millisecond {
		t.Errorf("PersistFlushInterval = %v, want 500ms", cfg.PersistFlushInterval)
	}
}

func TestLoad_EnvironmentVariables(t *testing.T) {
	t.Setenv("PORT", "9000")
	t.Setenv("DYNAMODB_ENDPOINT", "http://dynamodb:8000")
	t.Setenv("DYNAMODB_REGION", "us-west-2")
	t.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("PERSIST_FLUSH_INTERVAL", "250ms")
	t.Setenv("BRIEFING_ENABLED", "true")
	t.Setenv("ELEVENLABS_API_KEY", "k")
	t.Setenv("ELEVENLABS_VOICE_ID", "v")

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

	if cfg.PersistFlushInterval != 250*time.Millisecond {
		t.Errorf("PersistFlushInterval = %v, want 250ms", cfg.PersistFlushInterval)
	}
	if !cfg.BriefingEnabled {
		t.Error("BriefingEnabled = false, want true")
	}
	if problems := cfg.Validate(); len(problems) != 0 {
		t.Errorf("Validate() = %v, want none", problems)
	}
}

func TestLoad_PartialEnvironment(t *testing.T) {
	t.Setenv("PORT", "3000")
	t.Setenv("LOG_LEVEL", "error")

	cfg := Load()

	if cfg.Port != "3000" {
		t.Errorf("Port = %q, want %q", cfg.Port, "3000")
	}
	if cfg.LogLevel != "error" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "error")
	}
	if cfg.DynamoDBEndpoint != "http://localhost:8000" {
		t.Errorf("DynamoDBEndpoint = %q, want default", cfg.DynamoDBEndpoint)
	}
	if cfg.DynamoDBRegion != "us-east-1" {
		t.Errorf("DynamoDBRegion = %q, want default", cfg.DynamoDBRegion)
	}
}

func TestGetEnv_WithValue(t *testing.T) {
	t.Setenv("TEST_VAR", "test-value")

	result := getEnv("TEST_VAR", "default")
	if result != "test-value" {
		t.Errorf("getEnv() = %q, want %q", result, "test-value")
	}
}

func TestGetEnv_WithoutValue(t *testing.T) {
	// Rely on a key this suite never sets so os.Getenv returns "".
	result := getEnv("CONFIG_TEST_UNSET_VAR_XYZ", "default-value")
	if result != "default-value" {
		t.Errorf("getEnv() = %q, want %q", result, "default-value")
	}
}

func TestGetEnv_EmptyString(t *testing.T) {
	t.Setenv("EMPTY_VAR", "")

	result := getEnv("EMPTY_VAR", "default")
	if result != "default" {
		t.Errorf("getEnv() with empty string = %q, want %q", result, "default")
	}
}

func TestGetEnvInt_InvalidFallsBack(t *testing.T) {
	t.Setenv("PERSIST_WORKERS", "not-a-number")
	cfg := Load()
	if cfg.PersistWorkers != 4 {
		t.Errorf("PersistWorkers = %d, want default 4", cfg.PersistWorkers)
	}
}

func TestValidate_BriefingMissingCreds(t *testing.T) {
	cfg := &Config{BriefingEnabled: true}
	problems := cfg.Validate()
	if len(problems) != 2 {
		t.Fatalf("Validate() = %v, want 2 problems", problems)
	}
}
