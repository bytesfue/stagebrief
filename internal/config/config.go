package config

import (
	"errors"
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
	OpenAIAPIKey string
	OpenAIModel  string

	// Slack
	SlackBotToken string
	SlackChannel  string

	// Display
	ProjectName string
	StagingURL  string

	// Message config
	ShowChangedFiles bool
	ShowRawCommits   bool
	MaxFiles         int
	MaxCommits       int

	// Behaviour
	SkipPatterns []string
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

	cfg.OpenAIAPIKey = os.Getenv("OPENAI_API_KEY")
	if cfg.OpenAIAPIKey == "" {
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
		cfg.OpenAIModel = "gpt-4o-mini"
	}

	cfg.ProjectName = os.Getenv("GITLAB_PROJECT_NAME")
	if cfg.ProjectName == "" {
		cfg.ProjectName = cfg.GitLabProjectID
	}

	cfg.StagingURL = os.Getenv("STAGING_URL")

	cfg.ShowChangedFiles = envBool("SHOW_CHANGED_FILES", true)
	cfg.ShowRawCommits = envBool("SHOW_RAW_COMMITS", true)
	cfg.MaxFiles = envInt("MAX_FILES", 10)
	cfg.MaxCommits = envInt("MAX_COMMITS", 10)

	if patterns := os.Getenv("SKIP_PATTERNS"); patterns != "" {
		for _, p := range strings.Split(patterns, ",") {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				cfg.SkipPatterns = append(cfg.SkipPatterns, trimmed)
			}
		}
	}

	return cfg, nil
}

// Require returns the named env var or adds it to the missing list.
// For use in tests or future extensions.
func Require(name string) (string, error) {
	v := os.Getenv(name)
	if v == "" {
		return "", fmt.Errorf("%w: %s", errors.New("missing required env var"), name)
	}
	return v, nil
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
