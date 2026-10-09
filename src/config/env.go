package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type EnvConfig struct {
	AppPort            string
	DBHost             string
	DBPort             string
	DBUser             string
	DBPassword         string
	DBName             string
	DBSSLMode          string
	AIProvider         string
	AnthropicAPIKey    string
	AnthropicModel     string
	AnthropicBaseURL   string
	CORSAllowedOrigins string
	OpenAIAPIKey            string
	OpenAIMBaseURL          string
	OpenAIModel             string
	AIMaxTokens             int
	RedisURL                string
	InternalServiceKey      string
	OrchestrationServiceURL string
}

var Env *EnvConfig

func LoadEnv() {
	if err := godotenv.Load(); err != nil {
		if err2 := godotenv.Load("../../.env"); err2 != nil {
			if err3 := godotenv.Load("../.env"); err3 != nil {
				log.Println("Warning: .env file not found, using system environment variables")
			}
		}
	}

	Env = &EnvConfig{
		AppPort:            getEnv("APP_PORT", "8080"),
		DBHost:             getEnv("DB_HOST", "localhost"),
		DBPort:             getEnv("DB_PORT", "5432"),
		DBUser:             getEnv("DB_USER", "postgres"),
		DBPassword:         getEnv("DB_PASSWORD", ""),
		DBName:             getEnv("DB_NAME", "diagram_db"),
		DBSSLMode:          getEnv("DB_SSLMODE", "disable"),
		AIProvider:         getEnv("AI_PROVIDER", "openai"),
		AnthropicAPIKey:    getEnv("ANTHROPIC_API_KEY", ""),
		AnthropicModel:     getEnv("ANTHROPIC_MODEL", "claude-3-5-sonnet-20241022"),
		AnthropicBaseURL:   getEnv("ANTHROPIC_BASE_URL", "https://api.anthropic.com/v1/messages"),
		CORSAllowedOrigins: getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000"),
		OpenAIAPIKey:       getEnvAny([]string{"OPENAI_API_KEY", "OPEN_AI_API_KEY", "AI_API_KEY"}, "sk-27534e0917d892bb-z0aum1-3e0637a7"),
		OpenAIMBaseURL:     getEnvAny([]string{"OPENAI_BASE_URL", "OPEN_AI_BASE_URL", "AI_BASE_URL"}, "http://localhost:20128/v1"),
		OpenAIModel:             getEnvAny([]string{"OPENAI_MODEL", "OPEN_AI_MODEL", "AI_MODEL"}, "ag/gemini-3.8-flash"),
		// Cap output at 10,000 tokens: a single production-ready screen typically
		// fits in 3-5k. A large cap invites the model to over-generate, inflating
		// latency, token cost, and unrequested UI bloat.
		AIMaxTokens:             getEnvInt("AI_MAX_TOKENS", 10000),
		RedisURL:                getEnv("REDIS_URL", "redis://localhost:6379/0"),
		InternalServiceKey:      getEnv("INTERNAL_SERVICE_KEY", "secret-internal-key-project-diagram"),
		OrchestrationServiceURL: getEnv("ORCHESTRATION_SERVICE_URL", "http://localhost:8000"),
	}

	// Auto-replace localhost with host.docker.internal only if host.docker.internal is explicitly present in /etc/hosts (bridge mode)
	if hostsData, err := os.ReadFile("/etc/hosts"); err == nil && strings.Contains(string(hostsData), "host.docker.internal") {
		if strings.Contains(Env.OpenAIMBaseURL, "localhost") {
			Env.OpenAIMBaseURL = strings.ReplaceAll(Env.OpenAIMBaseURL, "localhost", "host.docker.internal")
		}
		if strings.Contains(Env.OpenAIMBaseURL, "127.0.0.1") {
			Env.OpenAIMBaseURL = strings.ReplaceAll(Env.OpenAIMBaseURL, "127.0.0.1", "host.docker.internal")
		}
	}

	log.Println("Environment variables loaded successfully")
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	if n, err := strconv.Atoi(value); err == nil && n > 0 {
		return n
	}
	return fallback
}

func getEnvAny(keys []string, fallback string) string {
	for _, key := range keys {
		if val := os.Getenv(key); val != "" {
			return val
		}
	}
	return fallback
}

