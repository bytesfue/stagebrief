package llm

import (
	"errors"
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
