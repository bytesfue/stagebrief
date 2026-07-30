package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bytesfue/stagingbrief/internal/httpretry"
)

const defaultClaudeModel = "claude-sonnet-5"
const defaultClaudeBaseURL = "https://api.anthropic.com/v1"
const anthropicVersion = "2023-06-01"

// claudeMaxTokens caps response length; the summarisation prompt asks for
// under 120 words, so this leaves ample headroom.
const claudeMaxTokens = 1024

// ClaudeClient talks to Anthropic's Messages API. It satisfies the same
// ChatCompleter interface as the OpenAI Client so callers can use either
// provider interchangeably.
type ClaudeClient struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
	retry      httpretry.Policy
}

// ClaudeOption configures optional ClaudeClient behaviour, e.g. for tests.
type ClaudeOption func(*ClaudeClient)

// WithClaudeBaseURL overrides the Anthropic API base URL. Intended for tests
// that point the client at a local httptest server; production callers
// should leave this unset to use the default Anthropic endpoint.
func WithClaudeBaseURL(baseURL string) ClaudeOption {
	return func(c *ClaudeClient) {
		c.baseURL = baseURL
	}
}

func NewClaudeClient(apiKey, model string, opts ...ClaudeOption) *ClaudeClient {
	if model == "" {
		model = defaultClaudeModel
	}

	c := &ClaudeClient{
		apiKey:  apiKey,
		model:   model,
		baseURL: defaultClaudeBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		retry: httpretry.DefaultPolicy(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system"`
	Messages  []claudeMessage `json:"messages"`
}

type claudeContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type claudeResponse struct {
	Content []claudeContentBlock `json:"content"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Pricing per 1000 tokens in USD — verify at https://platform.claude.com/docs/en/pricing
// Last updated: July 2026
var claudeModelPricing = map[string]struct {
	InputPer1k  float64
	OutputPer1k float64
}{
	"claude-sonnet-5": {InputPer1k: 0.003000, OutputPer1k: 0.015000},
	"claude-opus-5":   {InputPer1k: 0.005000, OutputPer1k: 0.025000},
}

func (c *ClaudeClient) ChatCompletion(systemPrompt, userPrompt string) (Result, error) {
	reqBody := claudeRequest{
		Model:     c.model,
		MaxTokens: claudeMaxTokens,
		System:    systemPrompt,
		Messages: []claudeMessage{
			{Role: "user", Content: userPrompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return Result{}, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.retry.Do(c.httpClient, func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodPost, c.baseURL+"/messages", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("x-api-key", c.apiKey)
		req.Header.Set("anthropic-version", anthropicVersion)
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	})
	if err != nil {
		return Result{}, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{}, fmt.Errorf("read response: %w", err)
	}

	var claudeResp claudeResponse
	if err := json.Unmarshal(respBody, &claudeResp); err != nil {
		return Result{}, fmt.Errorf("decode response: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		if claudeResp.Error != nil {
			return Result{}, fmt.Errorf("%w: %s", ErrQuotaExceeded, claudeResp.Error.Message)
		}
		return Result{}, ErrQuotaExceeded
	}

	if resp.StatusCode != http.StatusOK {
		if claudeResp.Error != nil {
			return Result{}, fmt.Errorf("%w: %s", ErrAPIError, claudeResp.Error.Message)
		}
		return Result{}, fmt.Errorf("%w: status %d", ErrAPIError, resp.StatusCode)
	}

	if len(claudeResp.Content) == 0 {
		return Result{}, fmt.Errorf("claude API returned no content")
	}

	cost := estimateClaudeCost(c.model, claudeResp.Usage.InputTokens, claudeResp.Usage.OutputTokens)

	return Result{
		Summary:          strings.TrimSpace(claudeResp.Content[0].Text),
		PromptTokens:     claudeResp.Usage.InputTokens,
		CompletionTokens: claudeResp.Usage.OutputTokens,
		TotalTokens:      claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
		EstimatedCostUSD: cost,
	}, nil
}

func estimateClaudeCost(model string, inputTokens, outputTokens int) float64 {
	pricing, ok := claudeModelPricing[model]
	if !ok {
		return 0
	}
	input := float64(inputTokens) / 1000 * pricing.InputPer1k
	output := float64(outputTokens) / 1000 * pricing.OutputPer1k
	return input + output
}
