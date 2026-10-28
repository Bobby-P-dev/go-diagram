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
	for _, name := range []string{"OPENAI_API_KEY", "OPEN_AI_API_KEY", "AI_API_KEY", "INTERNAL_SERVICE_KEY", "MASTER_ADMIN_KEY", "ANTHROPIC_API_KEY"} {
		t.Setenv(name, "")
	}
	env := LoadEnv()
	if env.OpenAIAPIKey != "" || env.InternalServiceKey != "" || env.MasterAdminKey != "" || env.AnthropicAPIKey != "" {
		t.Fatal("secrets must remain empty when not supplied")
	}
	for _, alias := range []string{"OPENAI_API_KEY", "OPEN_AI_API_KEY", "AI_API_KEY"} {
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
