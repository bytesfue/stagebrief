package main

import (
	"fmt"
	"log"
	"os"

	"github.com/bytesfue/stagingbrief/internal/gitlab"
	"github.com/joho/godotenv"
)

func main() {
	LoadEnv()

	token := os.Getenv("GITLAB_TOKEN")
	projectID := os.Getenv("GITLAB_PROJECT_ID")
	baseURL := os.Getenv("GITLAB_BASE_URL")
	currentCommitSHA := os.Getenv("CI_COMMIT_SHA")
	branch := os.Getenv("CI_COMMIT_BRANCH")

	if token == "" || projectID == "" || baseURL == "" || currentCommitSHA == "" || branch == "" {
		log.Fatal("missing environment variables")
	}

	client := gitlab.NewClient(token, baseURL)

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

}

func LoadEnv() {
	// Ignore errors: .env is optional.
	if err := godotenv.Load(); err == nil {
		log.Println("Loaded .env file")
	}
}
