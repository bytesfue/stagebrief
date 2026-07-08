package llm

import (
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestClient(baseURL string) *Client {
	return NewClient("test-key", defaultModel, WithBaseURL(baseURL))
}

func TestChatCompletion_RateLimitedWithoutErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	_, err := client.ChatCompletion("system", "user")
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("expected ErrQuotaExceeded, got %v", err)
	}
}

func TestChatCompletion_RateLimitedWithErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error": {"message": "quota exceeded for this key"}}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	_, err := client.ChatCompletion("system", "user")
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("expected ErrQuotaExceeded, got %v", err)
	}
	if err.Error() != "llm quota exceeded: quota exceeded for this key" {
		t.Fatalf("expected error message to include API message, got %q", err.Error())
	}
}

func TestEstimateCost(t *testing.T) {
	tests := []struct {
		name             string
		model            string
		promptTokens     int
		completionTokens int
		want             float64
	}{
		{
			name:             "known model with usage",
			model:            "gpt-4o-mini",
			promptTokens:     1000,
			completionTokens: 1000,
			want:             0.000150 + 0.000600,
		},
		{
			name:             "known model, zero tokens",
			model:            "gpt-4o-mini",
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
			name:             "gpt-4o pricing",
			model:            "gpt-4o",
			promptTokens:     2000,
			completionTokens: 500,
			want:             (2000.0/1000)*0.002500 + (500.0/1000)*0.010000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateCost(tt.model, tt.promptTokens, tt.completionTokens)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("estimateCost(%q, %d, %d) = %v, want %v", tt.model, tt.promptTokens, tt.completionTokens, got, tt.want)
			}
		})
	}
}
