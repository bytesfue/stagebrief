package config

import (
	"strings"
	"testing"
)

func TestEnvBool(t *testing.T) {
	tests := []struct {
		name       string
		envValue   string
		envUnset   bool
		defaultVal bool
		want       bool
	}{
		{name: "unset uses default true", envUnset: true, defaultVal: true, want: true},
		{name: "unset uses default false", envUnset: true, defaultVal: false, want: false},
		{name: "empty string uses default", envValue: "", defaultVal: true, want: true},
		{name: "lowercase false disables", envValue: "false", defaultVal: true, want: false},
		{name: "uppercase FALSE is NOT false (quirk)", envValue: "FALSE", defaultVal: true, want: true},
		{name: "numeric zero is NOT false (quirk)", envValue: "0", defaultVal: true, want: true},
		{name: "true enables", envValue: "true", defaultVal: false, want: true},
		{name: "arbitrary value is treated as true", envValue: "yes", defaultVal: false, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const key = "TEST_ENV_BOOL"
			if !tt.envUnset {
				t.Setenv(key, tt.envValue)
			}

			got := envBool(key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("envBool(%q, %v) = %v, want %v", tt.envValue, tt.defaultVal, got, tt.want)
			}
		})
	}
}

func TestEnvInt(t *testing.T) {
	tests := []struct {
		name       string
		envValue   string
		envUnset   bool
		defaultVal int
		want       int
	}{
		{name: "unset uses default", envUnset: true, defaultVal: 10, want: 10},
		{name: "empty string uses default", envValue: "", defaultVal: 10, want: 10},
		{name: "valid positive integer", envValue: "5", defaultVal: 10, want: 5},
		{name: "zero is valid", envValue: "0", defaultVal: 10, want: 0},
		{name: "invalid non-numeric uses default", envValue: "abc", defaultVal: 10, want: 10},
		{name: "negative value uses default", envValue: "-1", defaultVal: 10, want: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const key = "TEST_ENV_INT"
			if !tt.envUnset {
				t.Setenv(key, tt.envValue)
			}

			got := envInt(key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("envInt(%q, %d) = %d, want %d", tt.envValue, tt.defaultVal, got, tt.want)
			}
		})
	}
}

// requiredEnvVars are the env vars Load() treats as mandatory.
var requiredEnvVars = []string{
	"GITLAB_TOKEN",
	"GITLAB_PROJECT_ID",
	"CI_COMMIT_SHA",
	"CI_COMMIT_BRANCH",
	"OPENAI_API_KEY",
	"SLACK_BOT_TOKEN",
	"SLACK_CHANNEL_ID",
}

// optionalEnvVars are the optional env vars Load() reads, cleared in tests
// so defaults are deterministic regardless of the host shell's environment.
var optionalEnvVars = []string{
	"CI_API_V4_URL",
	"LLM_PROVIDER",
	"OPENAI_MODEL",
	"ANTHROPIC_API_KEY",
	"ANTHROPIC_MODEL",
	"GITLAB_PROJECT_NAME",
	"SHOW_CHANGED_FILES",
	"SHOW_RAW_COMMITS",
	"MAX_FILES",
	"MAX_COMMITS",
	"NOTIFY_ON_NO_CHANGES",
}

func clearOptionalEnvVars(t *testing.T) {
	t.Helper()
	for _, key := range optionalEnvVars {
		t.Setenv(key, "")
	}
}

func setAllRequiredEnvVars(t *testing.T) {
	t.Helper()
	for _, key := range requiredEnvVars {
		t.Setenv(key, "value-for-"+key)
	}
}

func TestLoad_MissingRequiredEnvVars(t *testing.T) {
	// Ensure every required var is unset/empty so all are reported missing.
	for _, key := range requiredEnvVars {
		t.Setenv(key, "")
	}
	clearOptionalEnvVars(t)

	cfg, err := Load()
	if cfg != nil {
		t.Fatalf("expected nil config on error, got %+v", cfg)
	}
	if err == nil {
		t.Fatal("expected error for missing required env vars, got nil")
	}

	for _, key := range requiredEnvVars {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("expected error to mention missing var %q, got: %v", key, err)
		}
	}
}

func TestLoad_PartiallyMissingRequiredEnvVars(t *testing.T) {
	setAllRequiredEnvVars(t)
	clearOptionalEnvVars(t)

	// Unset just one required var and confirm only it is reported.
	t.Setenv("SLACK_CHANNEL_ID", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing SLACK_CHANNEL_ID, got nil")
	}
	if !strings.Contains(err.Error(), "SLACK_CHANNEL_ID") {
		t.Errorf("expected error to mention SLACK_CHANNEL_ID, got: %v", err)
	}
	for _, key := range requiredEnvVars {
		if key == "SLACK_CHANNEL_ID" {
			continue
		}
		if strings.Contains(err.Error(), key) {
			t.Errorf("did not expect error to mention %q since it was set, got: %v", key, err)
		}
	}
}

func TestLoad_DefaultsForOptionalVars(t *testing.T) {
	setAllRequiredEnvVars(t)
	clearOptionalEnvVars(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GitLabAPIURL != "https://gitlab.com/api/v4" {
		t.Errorf("GitLabAPIURL = %q, want default", cfg.GitLabAPIURL)
	}
	if cfg.OpenAIModel != "gpt-5-mini" {
		t.Errorf("OpenAIModel = %q, want default", cfg.OpenAIModel)
	}
	if cfg.LLMProvider != "openai" {
		t.Errorf("LLMProvider = %q, want default %q", cfg.LLMProvider, "openai")
	}
	if cfg.AnthropicModel != "claude-haiku-4-5" {
		t.Errorf("AnthropicModel = %q, want default", cfg.AnthropicModel)
	}
	if cfg.AnthropicAPIKey != "" {
		t.Errorf("AnthropicAPIKey = %q, want empty when unset and provider is openai", cfg.AnthropicAPIKey)
	}
	if cfg.ProjectName != cfg.GitLabProjectID {
		t.Errorf("ProjectName = %q, want it to default to GitLabProjectID %q", cfg.ProjectName, cfg.GitLabProjectID)
	}
	if !cfg.ShowChangedFiles {
		t.Error("ShowChangedFiles = false, want default true")
	}
	if !cfg.ShowRawCommits {
		t.Error("ShowRawCommits = false, want default true")
	}
	if cfg.MaxFiles != 10 {
		t.Errorf("MaxFiles = %d, want default 10", cfg.MaxFiles)
	}
	if cfg.MaxCommits != 10 {
		t.Errorf("MaxCommits = %d, want default 10", cfg.MaxCommits)
	}
	if !cfg.NotifyOnNoChanges {
		t.Error("NotifyOnNoChanges = false, want default true")
	}
}

func TestLoad_OverridesForOptionalVars(t *testing.T) {
	setAllRequiredEnvVars(t)
	clearOptionalEnvVars(t)

	t.Setenv("CI_API_V4_URL", "https://gitlab.example.com/api/v4")
	t.Setenv("OPENAI_MODEL", "gpt-5")
	t.Setenv("ANTHROPIC_MODEL", "claude-sonnet-5")
	t.Setenv("GITLAB_PROJECT_NAME", "My Project")
	t.Setenv("SHOW_CHANGED_FILES", "false")
	t.Setenv("SHOW_RAW_COMMITS", "false")
	t.Setenv("MAX_FILES", "5")
	t.Setenv("MAX_COMMITS", "3")
	t.Setenv("NOTIFY_ON_NO_CHANGES", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GitLabAPIURL != "https://gitlab.example.com/api/v4" {
		t.Errorf("GitLabAPIURL = %q", cfg.GitLabAPIURL)
	}
	if cfg.OpenAIModel != "gpt-5" {
		t.Errorf("OpenAIModel = %q", cfg.OpenAIModel)
	}
	if cfg.AnthropicModel != "claude-sonnet-5" {
		t.Errorf("AnthropicModel = %q", cfg.AnthropicModel)
	}
	if cfg.ProjectName != "My Project" {
		t.Errorf("ProjectName = %q", cfg.ProjectName)
	}
	if cfg.ShowChangedFiles {
		t.Error("ShowChangedFiles = true, want false")
	}
	if cfg.ShowRawCommits {
		t.Error("ShowRawCommits = true, want false")
	}
	if cfg.MaxFiles != 5 {
		t.Errorf("MaxFiles = %d, want 5", cfg.MaxFiles)
	}
	if cfg.MaxCommits != 3 {
		t.Errorf("MaxCommits = %d, want 3", cfg.MaxCommits)
	}
	if cfg.NotifyOnNoChanges {
		t.Error("NotifyOnNoChanges = true, want false")
	}
}

func TestLoad_DefaultProviderRequiresOnlyOpenAIKey(t *testing.T) {
	setAllRequiredEnvVars(t)
	clearOptionalEnvVars(t)
	// ANTHROPIC_API_KEY intentionally left unset — must not be required
	// when LLM_PROVIDER is unset (defaults to openai).

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error with default provider and no ANTHROPIC_API_KEY: %v", err)
	}
	if cfg.LLMProvider != "openai" {
		t.Errorf("LLMProvider = %q, want %q", cfg.LLMProvider, "openai")
	}
}

func TestLoad_ClaudeProviderRequiresAnthropicKeyNotOpenAI(t *testing.T) {
	for _, key := range requiredEnvVars {
		if key == "OPENAI_API_KEY" {
			t.Setenv(key, "") // intentionally left unset — must not be required under claude
			continue
		}
		t.Setenv(key, "value-for-"+key)
	}
	clearOptionalEnvVars(t)
	t.Setenv("LLM_PROVIDER", "claude")
	t.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error with claude provider and no OPENAI_API_KEY: %v", err)
	}
	if cfg.LLMProvider != "claude" {
		t.Errorf("LLMProvider = %q, want %q", cfg.LLMProvider, "claude")
	}
	if cfg.AnthropicAPIKey != "test-anthropic-key" {
		t.Errorf("AnthropicAPIKey = %q, want %q", cfg.AnthropicAPIKey, "test-anthropic-key")
	}
}

func TestLoad_ClaudeProviderMissingAnthropicKey(t *testing.T) {
	setAllRequiredEnvVars(t)
	clearOptionalEnvVars(t)
	t.Setenv("LLM_PROVIDER", "claude")
	// ANTHROPIC_API_KEY intentionally left unset.

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing ANTHROPIC_API_KEY under claude provider, got nil")
	}
	if !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Errorf("expected error to mention ANTHROPIC_API_KEY, got: %v", err)
	}
	if strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Errorf("did not expect error to mention OPENAI_API_KEY under claude provider, got: %v", err)
	}
}
