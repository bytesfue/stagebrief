package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/bytesfue/stagingbrief/internal/gitlab"
	"github.com/bytesfue/stagingbrief/internal/llm"
	"github.com/bytesfue/stagingbrief/internal/slack"
	"github.com/joho/godotenv"
)

func main() {
	LoadEnv()

	token := os.Getenv("GITLAB_TOKEN")
	projectID := os.Getenv("GITLAB_PROJECT_ID")
	baseURL := os.Getenv("GITLAB_BASE_URL")
	currentCommitSHA := os.Getenv("CI_COMMIT_SHA")
	branch := os.Getenv("CI_COMMIT_BRANCH")
	projectName := os.Getenv("GITLAB_PROJECT_NAME")

	if token == "" || projectID == "" || baseURL == "" || currentCommitSHA == "" || branch == "" {
		log.Fatal("missing environment variables")
	}
	client := gitlab.NewClient(token, baseURL)

	botToken := os.Getenv("SLACK_BOT_TOKEN")
	channel := os.Getenv("SLACK_CHANNEL_ID")

	slackClient := slack.NewClient(botToken, channel)

	if botToken == "" || channel == "" {
		log.Fatal("missing required env vars: SLACK_BOT_TOKEN, SLACK_CHANNEL_ID")
	}

	lastSuccessfulPipelineSHA, err := client.GetLastSuccessfulPipelineSHA(projectID, branch, currentCommitSHA)
	if err != nil {
		log.Fatal("failed to retrieve last pipeline")
	}

	var commits []gitlab.Commit
	commits, err = client.GetCommitsBetween(projectID, lastSuccessfulPipelineSHA, currentCommitSHA)
	if err != nil {
		log.Fatalf("failed to retrieve commits: %w", err)
	}

	fmt.Printf("found %d commits since last deploy (%s):\n\n", len(commits), lastSuccessfulPipelineSHA[:8])
	for _, c := range commits {
		fmt.Printf("  %s  %s\n", c.ID[:8], c.Title)
	}

	if len(commits) > 0 {
		files, err := client.GetChangedFiles(projectID, lastSuccessfulPipelineSHA, currentCommitSHA)
		if err != nil {
			log.Fatalf("failed to retrieve changed files: %w", err)
		}

		fmt.Printf("\nchanged files (%d):\n\n", len(files))
		for _, file := range files {
			status := "M"
			switch {
			case file.NewFile:
				status = "A"
			case file.DeletedFile:
				status = "D"
			case file.RenamedFile:
				status = "R"
			}
			fmt.Printf("  %s  %s\n", status, file.NewPath)
		}

		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			log.Fatal("missing required env var: OPENAI_API_KEY")
		}
		model := os.Getenv("OPENAI_MODEL") // optional, defaults to gpt-4o-mini

		llmClient := llm.NewClient(apiKey, model)

		summary, err := llm.Summarise(llmClient, llm.Input{
			Commits: commits,
			Files:   files,
		})
		if err != nil {
			switch {
			case errors.Is(err, llm.ErrQuotaExceeded):
				summary = "⚠️ Could not generate summary — LLM quota exceeded. Check your API key billing."
			case errors.Is(err, llm.ErrAPIError):
				summary = fmt.Sprintf("⚠️ Could not generate summary — LLM API error: %v", err)
			default:
				summary = "⚠️ Could not generate summary — unexpected error. See CI logs for details."
				log.Printf("summarise error: %v", err) // still log it, just don't fatal
			}
		}

		fmt.Println("\n--- Summary ---")
		fmt.Println(summary)

		if err := slackClient.PostSummary(projectName, summary, commits, files, loadMessageConfig()); err != nil {
			log.Fatalf("post to slack: %v", err)
		}

		fmt.Println("✓ posted to slack")
	}
}

func LoadEnv() {
	// Ignore errors: .env is optional.
	if err := godotenv.Load(); err == nil {
		log.Println("Loaded .env file")
	}
}

func loadMessageConfig() slack.MessageConfig {
	cfg := slack.DefaultConfig()

	if v := os.Getenv("SHOW_CHANGED_FILES"); v == "false" {
		cfg.ShowChangedFiles = false
	}
	if v := os.Getenv("SHOW_RAW_COMMITS"); v == "false" {
		cfg.ShowRawCommits = false
	}
	if v := os.Getenv("MAX_FILES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.MaxFiles = n
		}
	}
	if v := os.Getenv("MAX_COMMITS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.MaxCommits = n
		}
	}

	return cfg
}
