package slack

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostSummary_HappyPath(t *testing.T) {
	var gotPayload payload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("expected Authorization header 'Bearer test-token', got %q", got)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	client := NewClient("test-token", "C123", WithBaseURL(server.URL))

	err := client.PostSummary("demo-project", "summary text", nil, nil, DefaultConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotPayload.Channel != "C123" {
		t.Errorf("expected channel C123, got %q", gotPayload.Channel)
	}
	if !strings.Contains(gotPayload.Text, "summary text") {
		t.Errorf("expected posted text to include summary, got: %q", gotPayload.Text)
	}
}

func TestPostSummary_APIErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slack always returns HTTP 200, with errors carried in the body.
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": false, "error": "channel_not_found"}`))
	}))
	defer server.Close()

	client := NewClient("test-token", "C123", WithBaseURL(server.URL))

	err := client.PostSummary("demo-project", "summary", nil, nil, DefaultConfig())
	if err == nil {
		t.Fatal("expected error for ok:false response, got nil")
	}
	if !strings.Contains(err.Error(), "channel_not_found") {
		t.Errorf("expected error to include Slack error code, got: %v", err)
	}
}

func TestPostSummary_NetworkErrorWhenServerUnreachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	unreachableURL := server.URL
	server.Close() // close immediately so the URL is unreachable

	// A network error is retryable — zero the backoff so it doesn't sleep.
	client := NewClient("test-token", "C123", WithBaseURL(unreachableURL))
	client.retry.BaseDelay = 0

	err := client.PostSummary("demo-project", "summary", nil, nil, DefaultConfig())
	if err == nil {
		t.Fatal("expected network error when server is unreachable, got nil")
	}
}

func TestPostSummary_RetriesServerErrorThenSucceeds(t *testing.T) {
	var count int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if count == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	client := NewClient("test-token", "C123", WithBaseURL(server.URL))
	client.retry.BaseDelay = 0

	if err := client.PostSummary("demo-project", "summary", nil, nil, DefaultConfig()); err != nil {
		t.Fatalf("expected success after a transient 503, got: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 attempts (503 then 200), got %d", count)
	}
}

func TestPostSummary_MalformedResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not valid json`))
	}))
	defer server.Close()

	client := NewClient("test-token", "C123", WithBaseURL(server.URL))

	err := client.PostSummary("demo-project", "summary", nil, nil, DefaultConfig())
	if err == nil {
		t.Fatal("expected decode error for malformed response body, got nil")
	}
}
