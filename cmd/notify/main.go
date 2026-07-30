package main

import (
	"errors"
	"flag"
	"fmt"
	"log"

	"github.com/bytesfue/stagingbrief/internal/config"
	"github.com/bytesfue/stagingbrief/internal/gitlab"
	"github.com/bytesfue/stagingbrief/internal/llm"
	"github.com/bytesfue/stagingbrief/internal/slack"
	"github.com/joho/godotenv"
)

// version is stamped at build time via:
//
//	go build -ldflags "-X main.version=v1.2.3"
//
// and defaults to "dev" for local, unstamped builds.
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	LoadEnv()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	messageConfig := slack.MessageConfig{
		ShowChangedFiles: cfg.ShowChangedFiles,
		ShowRawCommits:   cfg.ShowRawCommits,
		MaxFiles:         cfg.MaxFiles,
		MaxCommits:       cfg.MaxCommits,
	}

	gitlabClient := gitlab.NewClient(cfg.GitLabToken, cfg.GitLabAPIURL)
	slackClient := slack.NewClient(cfg.SlackBotToken, cfg.SlackChannel)

	var llmClient llm.ChatCompleter
	var llmModel string
	switch cfg.LLMProvider {
	case "claude":
		llmClient = llm.NewClaudeClient(cfg.AnthropicAPIKey, cfg.AnthropicModel)
		llmModel = cfg.AnthropicModel
	default:
		llmClient = llm.NewOpenAIClient(cfg.OpenAIAPIKey, cfg.OpenAIModel)
		llmModel = cfg.OpenAIModel
	}

	lastSuccessfulPipelineSHA, err := gitlabClient.GetLastSuccessfulPipelineSHA(cfg.GitLabProjectID, cfg.CommitBranch, cfg.CommitSHA)
	if err != nil {
		if errors.Is(err, gitlab.ErrNoPreviousPipeline) {
			fmt.Println("no previous successful pipeline — this looks like the first deploy")
			msg := "🚀 First deployment to staging — no prior deploy to compare against, so there's nothing to summarise yet."
			if err := slackClient.PostSummary(cfg.ProjectName, msg, nil, nil, messageConfig); err != nil {
				log.Fatalf("post to slack: %v", err)
			}
			fmt.Println("✓ posted to slack")
			return
		}
		log.Fatalf("failed to retrieve last pipeline: %v", err)
	}

	var commits []gitlab.Commit
	commits, err = gitlabClient.GetCommitsBetween(cfg.GitLabProjectID, lastSuccessfulPipelineSHA, cfg.CommitSHA)
	if err != nil {
		log.Fatalf("failed to retrieve commits: %v", err)
	}

	fmt.Printf("found %d commits since last deploy (%s):\n\n", len(commits), lastSuccessfulPipelineSHA[:8])

	if len(commits) == 0 {
		if !cfg.NotifyOnNoChanges {
			fmt.Println("no commits since last deploy — NOTIFY_ON_NO_CHANGES is false, skipping Slack notification")
			return
		}

		msg := "✅ Staging redeployed — no new commits since the last deploy."
		if err := slackClient.PostSummary(cfg.ProjectName, msg, nil, nil, messageConfig); err != nil {
			log.Fatalf("post to slack: %v", err)
		}
		fmt.Println("✓ posted to slack")
		return
	}

	files, err := gitlabClient.GetChangedFiles(cfg.GitLabProjectID, lastSuccessfulPipelineSHA, cfg.CommitSHA)
	if err != nil {
		log.Fatalf("failed to retrieve changed files: %v", err)
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

	log.Printf("LLM usage — provider: %s | model: %s | prompt: %d tokens | completion: %d tokens | total: %d tokens | estimated cost: $%.6f",
		cfg.LLMProvider,
		llmModel,
		result.PromptTokens,
		result.CompletionTokens,
		result.TotalTokens,
		result.EstimatedCostUSD,
	)

	if err := slackClient.PostSummary(cfg.ProjectName, result.Summary, commits, files, messageConfig); err != nil {
		log.Fatalf("post to slack: %v", err)
	}

	fmt.Println("✓ posted to slack")
}

func LoadEnv() {
	// Ignore errors: .env is optional.
	if err := godotenv.Load(); err == nil {
		log.Println("Loaded .env file")
	}
}
