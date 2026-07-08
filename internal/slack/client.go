package slack

import (
	"net/http"
	"time"
)

const defaultAPIURL = "https://slack.com/api/chat.postMessage"

type Client struct {
	botToken   string
	channel    string
	baseURL    string
	httpClient *http.Client
}

type apiResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

// Option configures optional Client behaviour, e.g. for tests.
type Option func(*Client)

// WithBaseURL overrides the Slack API URL. Intended for tests that point
// the client at a local httptest server; production callers should leave
// this unset to use the default Slack endpoint.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

func NewClient(botToken, channel string, opts ...Option) *Client {
	c := &Client{
		botToken: botToken,
		channel:  channel,
		baseURL:  defaultAPIURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}
