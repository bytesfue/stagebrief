package main

import (
	"fmt"
	"log"
	"os"

	"github.com/bytesfue/stagingbrief/internal/gitlab"
)

func main() {
	token := os.Getenv("GITLAB_TOKEN")
	projectID := os.Getenv("GITLAB_PROJECT_ID")
	baseURL := os.Getenv("GITLAB_BASE_URL")
	sha := os.Getenv("GITLAB_SHA")
	branch := os.Getenv("GITLAB_BRANCH")

	if token == "" || projectID == "" || baseURL == "" || sha == "" || branch == "" {
		log.Fatal("missing environment variables")
	}

	client := gitlab.NewClient(token, projectID)
	var commits []gitlab.Commit
	commits, err := client.GetCommitsBetween(projectID, branch, sha)
	if err != nil {
		log.Fatalf("failed to retrieve commits: %w", err)
	}

	for _, commit := range commits {
		fmt.Println(commit)
	}
}
