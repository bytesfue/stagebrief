package gitlab

import (
	"fmt"
	"net/url"
)

type Commit struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

type CompareResult struct {
	Commits []Commit   `json:"commits"`
	Diffs   []FileDiff `json:"diffs"`
}

type FileDiff struct {
	OldPath     string `json:"old_path"`
	NewPath     string `json:"new_path"`
	NewFile     bool   `json:"new_file"`
	RenamedFile bool   `json:"renamed_file"`
	DeletedFile bool   `json:"deleted_file"`
}

func (c *Client) GetCommitsBetween(projectID, fromSHA, toSHA string) ([]Commit, error) {
	// TODO: handle pagination
	path := fmt.Sprintf("/projects/%s/repository/commits?ref_name=%s..%s&per_page=50",
		url.QueryEscape(projectID),
		url.QueryEscape(fromSHA),
		url.QueryEscape(toSHA),
	)

	var commits []Commit
	if err := c.Get(path, &commits); err != nil {
		return nil, fmt.Errorf("failed to retrieve commits: %w", err)
	}

	return commits, nil
}

func (c *Client) GetChangedFiles(projectID, fromSHA, toSHA string) ([]FileDiff, error) {
	path := fmt.Sprintf("/projects/%s/repository/compare?from=%s&to=%s",
		url.QueryEscape(projectID),
		url.QueryEscape(fromSHA),
		url.QueryEscape(toSHA),
	)

	var result CompareResult
	if err := c.Get(path, &result); err != nil {
		return nil, fmt.Errorf("failed to retrieve file diffs: %w", err)
	}

	return result.Diffs, nil
}
