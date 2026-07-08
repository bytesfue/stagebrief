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
	basePath := fmt.Sprintf("/projects/%s/repository/commits?ref_name=%s..%s&per_page=100",
		url.QueryEscape(projectID),
		url.QueryEscape(fromSHA),
		url.QueryEscape(toSHA),
	)

	var all []Commit
	for page := 1; ; page++ {
		if page > maxPages {
			return nil, fmt.Errorf("exceeded maximum pagination depth (%d pages) fetching commits", maxPages)
		}

		var commits []Commit
		nextPage, err := c.getPage(fmt.Sprintf("%s&page=%d", basePath, page), &commits)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve commits: %w", err)
		}

		all = append(all, commits...)

		if nextPage == "" {
			break
		}
	}

	return all, nil
}

func (c *Client) GetChangedFiles(projectID, fromSHA, toSHA string) ([]FileDiff, error) {
	// straight=true requests a direct diff between fromSHA and toSHA (git diff from to)
	// rather than GitLab's default merge-base diff (git diff from...to). We want the
	// literal file-tree delta between the exact two deployed commits — merge-base
	// diffing recalculates against the common ancestor, which only matches fromSHA in
	// the simple fast-forward case and can mis-report changes if the branch was
	// rebased or force-pushed between deploys.
	path := fmt.Sprintf("/projects/%s/repository/compare?from=%s&to=%s&straight=true",
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
