package gitlab

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// maxPages caps how many pages a paginated GitLab list endpoint will be
// followed for, as a safety net against runaway loops (e.g. an
// unexpectedly huge repository or a misbehaving server). Var rather than
// const so tests can lower it to exercise the cap without waiting for 100
// real round-trips.
var maxPages = 100

type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

func NewClient(token, baseUrl string) *Client {
	return &Client{
		token:   token,
		baseURL: baseUrl,
		httpClient: &http.Client{
			Timeout: time.Second * 15,
		},
	}
}

func (c *Client) Get(path string, response any) error {
	_, err := c.getPage(path, response)
	return err
}

// getPage performs a GET request and also returns the value of the
// X-Next-Page response header, which GitLab list endpoints use to signal
// pagination continuation ("" means there is no further page).
func (c *Client) getPage(path string, response any) (nextPage string, err error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
		return "", err
	}

	return resp.Header.Get("X-Next-Page"), nil
}
