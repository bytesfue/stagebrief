package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	// GitLab
	GitLabToken     string
	GitLabProjectID string
	GitLabAPIURL    string

	// Git context — injected by GitLab CI
	CommitSHA    string
	CommitBranch string

	// LLM
	LLMProvider     string
	OpenAIAPIKey    string
	OpenAIModel     string
	AnthropicAPIKey string
	AnthropicModel  string

	// Slack
	SlackBotToken string
	SlackChannel  string

	// Display
	ProjectName string

	// Message config
	ShowChangedFiles bool
	ShowRawCommits   bool
	MaxFiles         int
	MaxCommits       int

	// Behaviour
	NotifyOnNoChanges bool
}

// Load reads all configuration from environment variables,
// validates required fields, and returns a populated Config.
func Load() (*Config, error) {
	cfg := &Config{}
	var missing []string

	// required
	cfg.GitLabToken = os.Getenv("GITLAB_TOKEN")
	if cfg.GitLabToken == "" {
		missing = append(missing, "GITLAB_TOKEN")
	}

	cfg.GitLabProjectID = os.Getenv("GITLAB_PROJECT_ID")
	if cfg.GitLabProjectID == "" {
		missing = append(missing, "GITLAB_PROJECT_ID")
	}

	cfg.CommitSHA = os.Getenv("CI_COMMIT_SHA")
	if cfg.CommitSHA == "" {
		missing = append(missing, "CI_COMMIT_SHA")
	}

	cfg.CommitBranch = os.Getenv("CI_COMMIT_BRANCH")
	if cfg.CommitBranch == "" {
		missing = append(missing, "CI_COMMIT_BRANCH")
	}

	cfg.LLMProvider = os.Getenv("LLM_PROVIDER")
	if cfg.LLMProvider == "" {
		cfg.LLMProvider = "openai"
	}

	cfg.OpenAIAPIKey = os.Getenv("OPENAI_API_KEY")
	cfg.AnthropicAPIKey = os.Getenv("ANTHROPIC_API_KEY")

	if cfg.LLMProvider == "claude" {
		if cfg.AnthropicAPIKey == "" {
			missing = append(missing, "ANTHROPIC_API_KEY")
		}
	} else if cfg.OpenAIAPIKey == "" {
		missing = append(missing, "OPENAI_API_KEY")
	}

	cfg.SlackBotToken = os.Getenv("SLACK_BOT_TOKEN")
	if cfg.SlackBotToken == "" {
		missing = append(missing, "SLACK_BOT_TOKEN")
	}

	cfg.SlackChannel = os.Getenv("SLACK_CHANNEL_ID")
	if cfg.SlackChannel == "" {
		missing = append(missing, "SLACK_CHANNEL_ID")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	// optional — with defaults
	cfg.GitLabAPIURL = os.Getenv("CI_API_V4_URL")
	if cfg.GitLabAPIURL == "" {
		cfg.GitLabAPIURL = "https://gitlab.com/api/v4"
	}

	cfg.OpenAIModel = os.Getenv("OPENAI_MODEL")
	if cfg.OpenAIModel == "" {
		cfg.OpenAIModel = "gpt-5-mini"
	}

	cfg.AnthropicModel = os.Getenv("ANTHROPIC_MODEL")
	if cfg.AnthropicModel == "" {
		cfg.AnthropicModel = "claude-haiku-4-5"
	}

	cfg.ProjectName = os.Getenv("GITLAB_PROJECT_NAME")
	if cfg.ProjectName == "" {
		cfg.ProjectName = cfg.GitLabProjectID
	}

	cfg.ShowChangedFiles = envBool("SHOW_CHANGED_FILES", true)
	cfg.ShowRawCommits = envBool("SHOW_RAW_COMMITS", true)
	cfg.MaxFiles = envInt("MAX_FILES", 10)
	cfg.MaxCommits = envInt("MAX_COMMITS", 10)

	cfg.NotifyOnNoChanges = envBool("NOTIFY_ON_NO_CHANGES", true)

	return cfg, nil
}

func envBool(key string, defaultVal bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	return v != "false"
}

func envInt(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return defaultVal
	}
	return n
}
