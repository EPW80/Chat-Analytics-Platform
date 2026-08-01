package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds all configuration for the application
type Config struct {
	Port             string
	DynamoDBEndpoint string
	DynamoDBRegion   string
	AWSAccessKey     string
	AWSSecretKey     string
	LogLevel         string

	// AllowedOrigins is the CORS/WebSocket origin allowlist. A single "*"
	// entry allows all origins (development default).
	AllowedOrigins []string

	// AuthSecret keys the HMAC token authenticator. Empty disables auth and
	// falls back to the userId query parameter (development only).
	AuthSecret string

	// RateLimitPerSec / RateLimitBurst tune the per-connection token bucket.
	// RateLimitPerSec <= 0 disables rate limiting.
	RateLimitPerSec float64
	RateLimitBurst  float64

	// Persistence worker pool tuning.
	PersistWorkers   int
	PersistBatchSize int
	PersistQueueSize int

	// Voice briefings. BriefingEnabled gates the whole feature: when false the
	// detector goroutine, briefing service and routes are never constructed.
	BriefingEnabled bool

	// ElevenLabs credentials and model for briefing text-to-speech.
	ElevenLabsKey   string
	ElevenLabsVoice string
	ElevenLabsModel string

	// BriefingAudioDir is where synthesized mp3s are written and cached. It must
	// live on a persisted volume or the content-hash cache is lost on restart.
	BriefingAudioDir string

	// BriefingCharBudget caps characters synthesized per month. Past the budget,
	// synthesis is refused and briefings degrade to text-only.
	BriefingCharBudget int
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

		AllowedOrigins: getEnvCSV("ALLOWED_ORIGINS", []string{"*"}),
		AuthSecret:     getEnv("AUTH_SECRET", ""),

		RateLimitPerSec: getEnvFloat("RATE_LIMIT_PER_SEC", 5),
		RateLimitBurst:  getEnvFloat("RATE_LIMIT_BURST", 10),

		PersistWorkers:   getEnvInt("PERSIST_WORKERS", 4),
		PersistBatchSize: getEnvInt("PERSIST_BATCH_SIZE", 25),
		PersistQueueSize: getEnvInt("PERSIST_QUEUE_SIZE", 1024),

		BriefingEnabled:    getEnvBool("BRIEFING_ENABLED", false),
		ElevenLabsKey:      getEnv("ELEVENLABS_API_KEY", ""),
		ElevenLabsVoice:    getEnv("ELEVENLABS_VOICE_ID", ""),
		ElevenLabsModel:    getEnv("ELEVENLABS_MODEL_ID", "eleven_flash_v2_5"),
		BriefingAudioDir:   getEnv("BRIEFING_AUDIO_DIR", "/data/briefings"),
		BriefingCharBudget: getEnvInt("BRIEFING_MONTHLY_CHAR_BUDGET", 10000),
	}
}

// getEnv reads an environment variable with a fallback default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt reads an integer environment variable, falling back on absence or
// a parse error.
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if n, err := strconv.Atoi(value); err == nil {
			return n
		}
	}
	return defaultValue
}

// getEnvFloat reads a float environment variable, falling back on absence or a
// parse error.
func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	}
	return defaultValue
}

// getEnvBool reads a boolean environment variable, falling back on absence or a
// parse error. Accepts the strconv.ParseBool set: 1/t/T/TRUE/true/True and the
// false equivalents.
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(strings.TrimSpace(value)); err == nil {
			return b
		}
	}
	return defaultValue
}

// getEnvCSV reads a comma-separated environment variable into a trimmed slice,
// falling back when unset or empty.
func getEnvCSV(key string, defaultValue []string) []string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return defaultValue
	}
	return out
}
