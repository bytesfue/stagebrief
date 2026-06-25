package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/bytesfue/stagingbrief/internal/config"
	"github.com/bytesfue/stagingbrief/internal/gitlab"
	"github.com/bytesfue/stagingbrief/internal/llm"
	"github.com/bytesfue/stagingbrief/internal/slack"
	"github.com/joho/godotenv"
)

func main() {
	LoadEnv()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	gitlabClient := gitlab.NewClient(cfg.GitLabToken, cfg.GitLabAPIURL)
	llmClient := llm.NewClient(cfg.OpenAIAPIKey, cfg.OpenAIModel)
	slackClient := slack.NewClient(cfg.SlackBotToken, cfg.SlackChannel)

	lastSuccessfulPipelineSHA, err := gitlabClient.GetLastSuccessfulPipelineSHA(cfg.GitLabProjectID, cfg.CommitBranch, cfg.CommitSHA)
	if err != nil {
		log.Fatal("failed to retrieve last pipeline")
	}

	var commits []gitlab.Commit
	commits, err = gitlabClient.GetCommitsBetween(cfg.GitLabProjectID, lastSuccessfulPipelineSHA, cfg.CommitSHA)
	if err != nil {
		log.Fatalf("failed to retrieve commits: %v", err)
	}

	fmt.Printf("found %d commits since last deploy (%s):\n\n", len(commits), lastSuccessfulPipelineSHA[:8])
	for _, c := range commits {
		fmt.Printf("  %s  %s\n", c.ID[:8], c.Title)
	}

	if len(commits) > 0 {
		files, err := gitlabClient.GetChangedFiles(cfg.GitLabProjectID, lastSuccessfulPipelineSHA, cfg.CommitSHA)
		if err != nil {
			log.Fatalf("failed to retrieve changed files: %v", err)
		}

		//fmt.Printf("\nchanged files (%d):\n\n", len(files))
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

		result, err := llm.Summarise(llmClient, llm.Input{
			Commits: commits,
			Files:   files,
		})
		if err != nil {
			switch {
			case errors.Is(err, llm.ErrQuotaExceeded):
				result.Summary = "⚠️ Could not generate summary — LLM quota exceeded. Check your API key billing."
			case errors.Is(err, llm.ErrAPIError):
				result.Summary = fmt.Sprintf("⚠️ Could not generate summary — LLM API error: %v", err)
			default:
				result.Summary = "⚠️ Could not generate summary — unexpected error. See CI logs for details."
				log.Printf("summarise error: %v", err)
			}
		}

		log.Printf("LLM usage — model: %s | prompt: %d tokens | completion: %d tokens | total: %d tokens | estimated cost: $%.6f",
			cfg.OpenAIModel,
			result.PromptTokens,
			result.CompletionTokens,
			result.TotalTokens,
			result.EstimatedCostUSD,
		)

		//fmt.Println("\n--- Summary ---")
		//fmt.Println(summary)

		if err := slackClient.PostSummary(cfg.ProjectName, result.Summary, commits, files, loadMessageConfig()); err != nil {
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
