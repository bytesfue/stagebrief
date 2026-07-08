package gitlab

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientGet_NonOKStatusReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message": "401 Unauthorized"}`))
	}))
	defer server.Close()

	client := NewClient("bad-token", server.URL)

	var out map[string]any
	err := client.Get("/projects/123", &out)
	if err == nil {
		t.Fatal("expected error for non-200 status, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected error to mention status code 401, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected error to include response body, got: %v", err)
	}
}

func TestClientGet_DecodeErrorOnMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not valid json`))
	}))
	defer server.Close()

	client := NewClient("test-token", server.URL)

	var out map[string]any
	err := client.Get("/projects/123", &out)
	if err == nil {
		t.Fatal("expected decode error for malformed JSON, got nil")
	}
}

func TestClientGet_SetsAuthHeader(t *testing.T) {
	var gotToken string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("PRIVATE-TOKEN")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewClient("my-secret-token", server.URL)

	var out map[string]any
	if err := client.Get("/projects/123", &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotToken != "my-secret-token" {
		t.Errorf("expected PRIVATE-TOKEN header to be set, got %q", gotToken)
	}
}
