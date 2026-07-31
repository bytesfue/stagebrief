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

const defaultOpenAIModel = "gpt-5-mini"
const defaultOpenAIBaseURL = "https://api.openai.com/v1"

// OpenAIClient talks to OpenAI's Chat Completions API. It satisfies the same
// ChatCompleter interface as ClaudeClient so callers can use either
// provider interchangeably.
type OpenAIClient struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
	retry      httpretry.Policy
}

// OpenAIOption configures optional OpenAIClient behaviour, e.g. for tests.
type OpenAIOption func(*OpenAIClient)

// WithOpenAIBaseURL overrides the OpenAI API base URL. Intended for tests
// that point the client at a local httptest server; production callers
// should leave this unset to use the default OpenAI endpoint.
func WithOpenAIBaseURL(baseURL string) OpenAIOption {
	return func(c *OpenAIClient) {
		c.baseURL = baseURL
	}
}

func NewOpenAIClient(apiKey, model string, opts ...OpenAIOption) *OpenAIClient {
	if model == "" {
		model = defaultOpenAIModel
	}

	c := &OpenAIClient{
		apiKey:  apiKey,
		model:   model,
		baseURL: defaultOpenAIBaseURL,
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

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
}

type openAIResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
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
// Last updated: July 2026
var openAIModelPricing = map[string]struct {
	InputPer1k  float64
	OutputPer1k float64
}{
	"gpt-5-mini": {InputPer1k: 0.000250, OutputPer1k: 0.002000},
	"gpt-5":      {InputPer1k: 0.001250, OutputPer1k: 0.010000},
}

func (c *OpenAIClient) ChatCompletion(systemPrompt, userPrompt string) (Result, error) {
	// No temperature field: the gpt-5 family only accepts the default (1)
	// and rejects any other value with a 400 unsupported_value error.
	reqBody := openAIRequest{
		Model: c.model,
		Messages: []openAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return Result{}, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.retry.Do(c.httpClient, func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
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

	var openAIResp openAIResponse
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return Result{}, fmt.Errorf("decode response: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		if openAIResp.Error != nil {
			return Result{}, fmt.Errorf("%w: %s", ErrQuotaExceeded, openAIResp.Error.Message)
		}
		return Result{}, ErrQuotaExceeded
	}

	if resp.StatusCode != http.StatusOK {
		if openAIResp.Error != nil {
			return Result{}, fmt.Errorf("%w: %s", ErrAPIError, openAIResp.Error.Message)
		}
		return Result{}, fmt.Errorf("%w: status %d", ErrAPIError, resp.StatusCode)
	}

	if len(openAIResp.Choices) == 0 {
		return Result{}, fmt.Errorf("openai API returned no choices")
	}

	cost := estimateOpenAICost(c.model, openAIResp.Usage.PromptTokens, openAIResp.Usage.CompletionTokens)

	return Result{
		Summary:          strings.TrimSpace(openAIResp.Choices[0].Message.Content),
		PromptTokens:     openAIResp.Usage.PromptTokens,
		CompletionTokens: openAIResp.Usage.CompletionTokens,
		TotalTokens:      openAIResp.Usage.TotalTokens,
		EstimatedCostUSD: cost,
	}, nil
}

func estimateOpenAICost(model string, promptTokens, completionTokens int) float64 {
	pricing, ok := openAIModelPricing[model]
	if !ok {
		return 0
	}
	input := float64(promptTokens) / 1000 * pricing.InputPer1k
	output := float64(completionTokens) / 1000 * pricing.OutputPer1k
	return input + output
}
