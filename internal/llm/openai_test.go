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

func newTestOpenAIClient(baseURL string) *OpenAIClient {
	c := NewOpenAIClient("test-key", defaultOpenAIModel, WithOpenAIBaseURL(baseURL))
	// Zero the backoff so retry-triggering tests (5xx) don't sleep for real.
	c.retry.BaseDelay = 0
	c.retry.MaxDelay = 0
	return c
}

func TestOpenAIChatCompletion_RateLimitedWithoutErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := newTestOpenAIClient(server.URL)

	_, err := client.ChatCompletion("system", "user")
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("expected ErrQuotaExceeded, got %v", err)
	}
}

func TestOpenAIChatCompletion_RateLimitedWithErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error": {"message": "quota exceeded for this key"}}`))
	}))
	defer server.Close()

	client := newTestOpenAIClient(server.URL)

	_, err := client.ChatCompletion("system", "user")
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("expected ErrQuotaExceeded, got %v", err)
	}
	if err.Error() != "llm quota exceeded: quota exceeded for this key" {
		t.Fatalf("expected error message to include API message, got %q", err.Error())
	}
}

func TestOpenAIChatCompletion_RetriesServerErrorThenSucceeds(t *testing.T) {
	var count int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices": [{"message": {"role": "assistant", "content": "recovered"}}]}`))
	}))
	defer server.Close()

	client := newTestOpenAIClient(server.URL)

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

func TestOpenAIChatCompletion_HappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [{"message": {"role": "assistant", "content": "  Here is your summary.  "}}],
			"usage": {"prompt_tokens": 100, "completion_tokens": 50, "total_tokens": 150}
		}`))
	}))
	defer server.Close()

	client := newTestOpenAIClient(server.URL)

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

func TestOpenAIChatCompletion_NoChoicesReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices": [], "usage": {"prompt_tokens": 10, "completion_tokens": 0, "total_tokens": 10}}`))
	}))
	defer server.Close()

	client := newTestOpenAIClient(server.URL)

	_, err := client.ChatCompletion("system", "user")
	if err == nil {
		t.Fatal("expected error when API returns no choices, got nil")
	}
}

func TestOpenAIChatCompletion_GenericAPIErrorWithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": {"message": "internal server error"}}`))
	}))
	defer server.Close()

	client := newTestOpenAIClient(server.URL)

	_, err := client.ChatCompletion("system", "user")
	if !errors.Is(err, ErrAPIError) {
		t.Fatalf("expected ErrAPIError, got %v", err)
	}
	if !strings.Contains(err.Error(), "internal server error") {
		t.Errorf("expected error to include API message, got: %v", err)
	}
}

func TestOpenAIChatCompletion_GenericAPIErrorWithoutBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := newTestOpenAIClient(server.URL)

	_, err := client.ChatCompletion("system", "user")
	if !errors.Is(err, ErrAPIError) {
		t.Fatalf("expected ErrAPIError, got %v", err)
	}
	if !strings.Contains(err.Error(), "502") {
		t.Errorf("expected error to mention status code 502, got: %v", err)
	}
}

func TestOpenAIChatCompletion_SendsExpectedRequest(t *testing.T) {
	var gotBody openAIRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("expected Authorization header 'Bearer test-key', got %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices": [{"message": {"role": "assistant", "content": "ok"}}]}`))
	}))
	defer server.Close()

	client := newTestOpenAIClient(server.URL)

	if _, err := client.ChatCompletion("system prompt text", "user prompt text"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotBody.Model != defaultOpenAIModel {
		t.Errorf("expected model %q, got %q", defaultOpenAIModel, gotBody.Model)
	}
	if len(gotBody.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(gotBody.Messages))
	}
	if gotBody.Messages[0].Role != "system" || gotBody.Messages[0].Content != "system prompt text" {
		t.Errorf("unexpected system message: %+v", gotBody.Messages[0])
	}
	if gotBody.Messages[1].Role != "user" || gotBody.Messages[1].Content != "user prompt text" {
		t.Errorf("unexpected user message: %+v", gotBody.Messages[1])
	}
}

func TestEstimateOpenAICost(t *testing.T) {
	tests := []struct {
		name             string
		model            string
		promptTokens     int
		completionTokens int
		want             float64
	}{
		{
			name:             "known model with usage",
			model:            "gpt-5-mini",
			promptTokens:     1000,
			completionTokens: 1000,
			want:             0.000250 + 0.002000,
		},
		{
			name:             "known model, zero tokens",
			model:            "gpt-5-mini",
			promptTokens:     0,
			completionTokens: 0,
			want:             0,
		},
		{
			name:             "unknown model returns zero",
			model:            "some-future-model",
			promptTokens:     1000,
			completionTokens: 1000,
			want:             0,
		},
		{
			name:             "gpt-5 pricing",
			model:            "gpt-5",
			promptTokens:     2000,
			completionTokens: 500,
			want:             (2000.0/1000)*0.001250 + (500.0/1000)*0.010000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateOpenAICost(tt.model, tt.promptTokens, tt.completionTokens)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("estimateOpenAICost(%q, %d, %d) = %v, want %v", tt.model, tt.promptTokens, tt.completionTokens, got, tt.want)
			}
		})
	}
}

// Compile-time check that OpenAIClient satisfies the ChatCompleter interface.
var _ ChatCompleter = (*OpenAIClient)(nil)
