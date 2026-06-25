package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultModel = "gpt-4o-mini"
const defaultBaseURL = "https://api.openai.com/v1"

type Client struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

func NewClient(apiKey, model string) *Client {
	if model == "" {
		model = defaultModel
	}

	return &Client{
		apiKey:  apiKey,
		model:   model,
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type Result struct {
	Summary          string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	EstimatedCostUSD float64
}

// Pricing per 1000 tokens in USD — verify at https://openai.com/pricing
// Last updated: June 2026
var modelPricing = map[string]struct {
	InputPer1k  float64
	OutputPer1k float64
}{
	"gpt-4o-mini": {InputPer1k: 0.000150, OutputPer1k: 0.000600},
	"gpt-4o":      {InputPer1k: 0.002500, OutputPer1k: 0.010000},
}

func (c *Client) ChatCompletion(systemPrompt, userPrompt string) (Result, error) {
	reqBody := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.3,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return Result{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Result{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{}, fmt.Errorf("read response: %w", err)
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return Result{}, fmt.Errorf("decode response: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == 429 {
		return Result{}, fmt.Errorf("%w: %s", ErrQuotaExceeded, chatResp.Error.Message)
	}

	if resp.StatusCode != http.StatusOK {
		if chatResp.Error != nil {
			return Result{}, fmt.Errorf("%w: %s", ErrAPIError, chatResp.Error.Message)
		}
		return Result{}, fmt.Errorf("%w: status %d", ErrAPIError, resp.StatusCode)
	}

	if len(chatResp.Choices) == 0 {
		return Result{}, fmt.Errorf("openai API returned no choices")
	}

	cost := estimateCost(c.model, chatResp.Usage.PromptTokens, chatResp.Usage.CompletionTokens)

	return Result{
		Summary:          strings.TrimSpace(chatResp.Choices[0].Message.Content),
		PromptTokens:     chatResp.Usage.PromptTokens,
		CompletionTokens: chatResp.Usage.CompletionTokens,
		TotalTokens:      chatResp.Usage.TotalTokens,
		EstimatedCostUSD: cost,
	}, nil
}

func estimateCost(model string, promptTokens, completionTokens int) float64 {
	pricing, ok := modelPricing[model]
	if !ok {
		return 0
	}
	input := float64(promptTokens) / 1000 * pricing.InputPer1k
	output := float64(completionTokens) / 1000 * pricing.OutputPer1k
	return input + output
}
