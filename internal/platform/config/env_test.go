package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvSecretsHaveNoFallback(t *testing.T) {
	workingDirectory := filepath.Join(t.TempDir(), "workspace", "config")
	if err := os.MkdirAll(workingDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workingDirectory)
	for _, name := range []string{"OPENAI_API_KEY", "OPEN_AI_API_KEY", "AI_API_KEY", "MODEL_API_KEY", "META_API_KEY", "INTERNAL_SERVICE_KEY", "MASTER_ADMIN_KEY", "ANTHROPIC_API_KEY"} {
		t.Setenv(name, "")
	}
	env := LoadEnv()
	if env.OpenAIAPIKey != "" || env.InternalServiceKey != "" || env.MasterAdminKey != "" || env.AnthropicAPIKey != "" {
		t.Fatal("secrets must remain empty when not supplied")
	}
	for _, alias := range []string{"OPENAI_API_KEY", "OPEN_AI_API_KEY", "AI_API_KEY", "MODEL_API_KEY", "META_API_KEY"} {
		t.Run(alias, func(t *testing.T) {
			t.Setenv(alias, "test-provider-value")
			env := LoadEnv()
			if env.OpenAIAPIKey != "test-provider-value" {
				t.Fatal("explicit provider key was not loaded")
			}
		})
	}
}

func TestEnvConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		env     EnvConfig
		wantErr bool
	}{
		{
			name:    "openai valid",
			env:     EnvConfig{AIProvider: "openai", OpenAIAPIKey: "provider-key", InternalServiceKey: "internal-key"},
			wantErr: false,
		},
		{
			name:    "anthropic valid",
			env:     EnvConfig{AIProvider: "anthropic", AnthropicAPIKey: "provider-key", InternalServiceKey: "internal-key"},
			wantErr: false,
		},
		{
			name:    "missing internal service key",
			env:     EnvConfig{AIProvider: "openai", OpenAIAPIKey: "provider-key"},
			wantErr: true,
		},
		{
			name:    "missing openai key",
			env:     EnvConfig{AIProvider: "openai", InternalServiceKey: "internal-key"},
			wantErr: true,
		},
		{
			name:    "missing anthropic key",
			env:     EnvConfig{AIProvider: "anthropic", InternalServiceKey: "internal-key"},
			wantErr: true,
		},
		{
			name:    "whitespace internal key",
			env:     EnvConfig{AIProvider: "openai", OpenAIAPIKey: "provider-key", InternalServiceKey: " \t"},
			wantErr: true,
		},
		{
			name:    "whitespace provider key",
			env:     EnvConfig{AIProvider: "openai", OpenAIAPIKey: " \t", InternalServiceKey: "internal-key"},
			wantErr: true,
		},
		{
			name:    "unsupported provider",
			env:     EnvConfig{AIProvider: "other", OpenAIAPIKey: "provider-key", InternalServiceKey: "internal-key"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.env.Validate(); (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadEnvMetaConfigurationAndPrecedence(t *testing.T) {
	workingDirectory := filepath.Join(t.TempDir(), "workspace", "config")
	if err := os.MkdirAll(workingDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workingDirectory)
	for _, name := range []string{
		"OPENAI_API_KEY", "OPEN_AI_API_KEY", "AI_API_KEY", "MODEL_API_KEY", "META_API_KEY",
		"OPENAI_MODEL", "OPEN_AI_MODEL", "AI_MODEL", "META_MODEL",
		"OPENAI_BASE_URL", "OPEN_AI_BASE_URL", "AI_BASE_URL", "META_BASE_URL",
	} {
		t.Setenv(name, "")
	}
	t.Setenv("AI_PROVIDER", "openai")
	t.Setenv("INTERNAL_SERVICE_KEY", "mock-internal-key")
	t.Setenv("MODEL_API_KEY", "mock-meta-key")
	t.Setenv("OPEN_AI_MODEL", "muse-spark-1.3")
	t.Setenv("OPEN_AI_BASE_URL", "https://api.meta.ai/v1")
	env := LoadEnv()
	if env.OpenAIAPIKey != "mock-meta-key" || env.OpenAIModel != "muse-spark-1.3" || env.OpenAIMBaseURL != "https://api.meta.ai/v1" {
		t.Fatal("Meta configuration was not resolved")
	}
	if err := env.Validate(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENAI_API_KEY", "mock-legacy-key")
	t.Setenv("OPENAI_MODEL", "legacy-model")
	t.Setenv("OPENAI_BASE_URL", "https://legacy.invalid/v1")
	env = LoadEnv()
	if env.OpenAIAPIKey != "mock-legacy-key" || env.OpenAIModel != "legacy-model" || env.OpenAIMBaseURL != "https://legacy.invalid/v1" {
		t.Fatal("existing OpenAI aliases must take precedence")
	}
}
