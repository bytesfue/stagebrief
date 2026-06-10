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

func (c *Client) GetCommitsBetween(projectID, fromSHA, toSHA string) ([]Commit, error) {
	path := fmt.Sprintf("projects/%s/commits?ref_name%s&since_sha=%s&per_page=50",
		url.QueryEscape(projectID), url.QueryEscape(fromSHA), url.QueryEscape(toSHA))

	var commits []Commit
	if err := c.Get(path, &commits); err != nil {
		return nil, fmt.Errorf("failed to retrieve commits: %w", err)
	}
	
	return commits, nil
}
