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

func (c *Client) GetCommitsBetween(projectID, fromSHA, branch string) ([]Commit, error) {
	path := fmt.Sprintf("/projects/%s/repository/commits?ref_name%s..%s&per_page=50",
		url.QueryEscape(projectID), url.QueryEscape(branch), url.QueryEscape(fromSHA))

	var commits []Commit
	if err := c.Get(path, &commits); err != nil {
		return nil, fmt.Errorf("failed to retrieve commits: %w", err)
	}

	return commits, nil
}
