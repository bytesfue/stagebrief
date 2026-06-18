package slack

import (
	"net/http"
	"time"
)

const apiURL = "https://slack.com/api/chat.postMessage"

type Client struct {
	botToken   string
	channel    string
	httpClient *http.Client
}

type apiResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

func NewClient(botToken, channel string) *Client {
	return &Client{
		botToken: botToken,
		channel:  channel,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}
