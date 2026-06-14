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
}

func LoadEnv() {
	// Ignore errors: .env is optional.
	if err := godotenv.Load(); err == nil {
		log.Println("Loaded .env file")
	}
}
