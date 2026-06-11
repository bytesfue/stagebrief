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
	sha := os.Getenv("CI_COMMIT_SHA")
	branch := os.Getenv("CI_COMMIT_BRANCH")

	if token == "" || projectID == "" || baseURL == "" || sha == "" || branch == "" {
		log.Fatal("missing environment variables")
	}

	client := gitlab.NewClient(token, baseURL)
	var commits []gitlab.Commit
	commits, err := client.GetCommitsBetween(projectID, branch, sha)
	if err != nil {
		log.Fatalf("failed to retrieve commits: %w", err)
	}

	for _, commit := range commits {
		fmt.Println(commit)
	}
}

func LoadEnv() {
	// Ignore errors: .env is optional.
	if err := godotenv.Load(); err == nil {
		log.Println("Loaded .env file")
	}
}
