package config

import (
	"log"
	"os"

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
	OpenAIAPIKey       string
	OpenAIMBaseURL     string
	OpenAIModel        string
}

var Env *EnvConfig

func LoadEnv() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
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
		OpenAIAPIKey:       getEnvAny([]string{"OPENAI_API_KEY", "AI_API_KEY"}, ""),
		OpenAIMBaseURL:     getEnvAny([]string{"OPENAI_BASE_URL", "AI_BASE_URL"}, "http://localhost:20128/v1"),
		OpenAIModel:        getEnvAny([]string{"OPENAI_MODEL", "AI_MODEL"}, "ag/gemini-3.8-flash-high"),
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

func getEnvAny(keys []string, fallback string) string {
	for _, key := range keys {
		if val := os.Getenv(key); val != "" {
			return val
		}
	}
	return fallback
}

