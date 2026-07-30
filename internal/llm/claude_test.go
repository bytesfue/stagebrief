package llm

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClaudeClient(baseURL string) *ClaudeClient {
	c := NewClaudeClient("test-key", defaultClaudeModel, WithClaudeBaseURL(baseURL))
	// Zero the backoff so retry-triggering tests (5xx) don't sleep for real.
	c.retry.BaseDelay = 0
	c.retry.MaxDelay = 0
	return c
}

func TestClaudeChatCompletion_RateLimitedWithoutErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := newTestClaudeClient(server.URL)

	_, err := client.ChatCompletion("system", "user")
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("expected ErrQuotaExceeded, got %v", err)
	}
}

func TestClaudeChatCompletion_RateLimitedWithErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error": {"type": "rate_limit_error", "message": "quota exceeded for this key"}}`))
	}))
	defer server.Close()

	client := newTestClaudeClient(server.URL)

	_, err := client.ChatCompletion("system", "user")
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("expected ErrQuotaExceeded, got %v", err)
	}
	if err.Error() != "llm quota exceeded: quota exceeded for this key" {
		t.Fatalf("expected error message to include API message, got %q", err.Error())
	}
}

func TestClaudeChatCompletion_RetriesServerErrorThenSucceeds(t *testing.T) {
	var count int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"content": [{"type": "text", "text": "recovered"}]}`))
	}))
	defer server.Close()

	client := newTestClaudeClient(server.URL)

	result, err := client.ChatCompletion("system", "user")
	if err != nil {
		t.Fatalf("expected success after a transient 500, got: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 attempts (500 then 200), got %d", count)
	}
	if result.Summary != "recovered" {
		t.Errorf("expected summary from the successful retry, got %q", result.Summary)
	}
}

func TestClaudeChatCompletion_HappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"content": [{"type": "text", "text": "  Here is your summary.  "}],
			"usage": {"input_tokens": 100, "output_tokens": 50}
		}`))
	}))
	defer server.Close()

	client := newTestClaudeClient(server.URL)

	result, err := client.ChatCompletion("system prompt", "user prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Summary != "Here is your summary." {
		t.Errorf("expected trimmed summary, got %q", result.Summary)
	}
	if result.PromptTokens != 100 || result.CompletionTokens != 50 || result.TotalTokens != 150 {
		t.Errorf("unexpected token counts: %+v", result)
	}
	if result.EstimatedCostUSD <= 0 {
		t.Errorf("expected non-zero estimated cost for known model with usage, got %v", result.EstimatedCostUSD)
	}
}

func TestClaudeChatCompletion_NoContentReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"content": [], "usage": {"input_tokens": 10, "output_tokens": 0}}`))
	}))
	defer server.Close()

	client := newTestClaudeClient(server.URL)

	_, err := client.ChatCompletion("system", "user")
	if err == nil {
		t.Fatal("expected error when API returns no content, got nil")
	}
}

func TestClaudeChatCompletion_GenericAPIErrorWithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": {"type": "api_error", "message": "internal server error"}}`))
	}))
	defer server.Close()

	client := newTestClaudeClient(server.URL)

	_, err := client.ChatCompletion("system", "user")
	if !errors.Is(err, ErrAPIError) {
		t.Fatalf("expected ErrAPIError, got %v", err)
	}
	if !strings.Contains(err.Error(), "internal server error") {
		t.Errorf("expected error to include API message, got: %v", err)
	}
}

func TestClaudeChatCompletion_GenericAPIErrorWithoutBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := newTestClaudeClient(server.URL)

	_, err := client.ChatCompletion("system", "user")
	if !errors.Is(err, ErrAPIError) {
		t.Fatalf("expected ErrAPIError, got %v", err)
	}
	if !strings.Contains(err.Error(), "502") {
		t.Errorf("expected error to mention status code 502, got: %v", err)
	}
}

func TestClaudeChatCompletion_SendsExpectedRequest(t *testing.T) {
	var gotBody claudeRequest
	var gotAPIKeyHeader, gotVersionHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKeyHeader = r.Header.Get("x-api-key")
		gotVersionHeader = r.Header.Get("anthropic-version")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"content": [{"type": "text", "text": "ok"}]}`))
	}))
	defer server.Close()

	client := newTestClaudeClient(server.URL)

	if _, err := client.ChatCompletion("system prompt text", "user prompt text"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotAPIKeyHeader != "test-key" {
		t.Errorf("expected x-api-key header 'test-key', got %q", gotAPIKeyHeader)
	}
	if gotVersionHeader != anthropicVersion {
		t.Errorf("expected anthropic-version header %q, got %q", anthropicVersion, gotVersionHeader)
	}
	if gotBody.Model != defaultClaudeModel {
		t.Errorf("expected model %q, got %q", defaultClaudeModel, gotBody.Model)
	}
	if gotBody.System != "system prompt text" {
		t.Errorf("expected system prompt %q, got %q", "system prompt text", gotBody.System)
	}
	if len(gotBody.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(gotBody.Messages))
	}
	if gotBody.Messages[0].Role != "user" || gotBody.Messages[0].Content != "user prompt text" {
		t.Errorf("unexpected user message: %+v", gotBody.Messages[0])
	}
}

func TestEstimateClaudeCost(t *testing.T) {
	tests := []struct {
		name         string
		model        string
		inputTokens  int
		outputTokens int
		want         float64
	}{
		{
			name:         "known model with usage",
			model:        "claude-haiku-4-5",
			inputTokens:  1000,
			outputTokens: 1000,
			want:         0.001000 + 0.005000,
		},
		{
			name:         "known model, zero tokens",
			model:        "claude-haiku-4-5",
			inputTokens:  0,
			outputTokens: 0,
			want:         0,
		},
		{
			name:         "unknown model returns zero",
			model:        "some-future-model",
			inputTokens:  1000,
			outputTokens: 1000,
			want:         0,
		},
		{
			name:         "claude-sonnet-5 pricing",
			model:        "claude-sonnet-5",
			inputTokens:  2000,
			outputTokens: 500,
			want:         (2000.0/1000)*0.003000 + (500.0/1000)*0.015000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateClaudeCost(tt.model, tt.inputTokens, tt.outputTokens)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("estimateClaudeCost(%q, %d, %d) = %v, want %v", tt.model, tt.inputTokens, tt.outputTokens, got, tt.want)
			}
		})
	}
}

// Compile-time check that ClaudeClient satisfies the ChatCompleter interface.
var _ ChatCompleter = (*ClaudeClient)(nil)
