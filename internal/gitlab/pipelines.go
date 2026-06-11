package gitlab

import (
	"fmt"
	"net/url"
	"time"
)

type Pipeline struct {
	ID        int       `json:"id"`
	Iid       int       `json:"iid"`
	ProjectID int       `json:"project_id"`
	Status    string    `json:"status"`
	Source    string    `json:"source"`
	Ref       string    `json:"ref"`
	Sha       string    `json:"sha"`
	Name      string    `json:"name"`
	WebURL    string    `json:"web_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (c *Client) GetLastSuccessfulPipelineSHA(projectID, branch, currentSHA string) (string, error) {
	path := fmt.Sprintf("/projects/%s/pipelines/?ref=%s&status=success&order_by=id&sort=desc&per_page=20",
		url.QueryEscape(projectID),
		url.QueryEscape(branch),
	)

	var pipelines []Pipeline
	if err := c.Get(path, &pipelines); err != nil {
		return "", fmt.Errorf("failed to get pipelines: %w", err)
	}

	// get the previous pipeline before the current one
	for _, pipeline := range pipelines {
		if pipeline.Sha != currentSHA {
			return pipeline.Sha, nil
		}
	}

	return "", fmt.Errorf("no previous pipeline found")
}
