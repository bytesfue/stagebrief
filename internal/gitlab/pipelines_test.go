package gitlab

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetLastSuccessfulPipelineSHA_NoPreviousPipeline(t *testing.T) {
	const currentSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id": 1, "sha": "` + currentSHA + `"}]`))
	}))
	defer server.Close()

	client := NewClient("test-token", server.URL)

	_, err := client.GetLastSuccessfulPipelineSHA("123", "develop", currentSHA)
	if !errors.Is(err, ErrNoPreviousPipeline) {
		t.Fatalf("expected ErrNoPreviousPipeline, got %v", err)
	}
}

func TestGetLastSuccessfulPipelineSHA_FindsPrevious(t *testing.T) {
	const currentSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const previousSHA = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id": 2, "sha": "` + currentSHA + `"}, {"id": 1, "sha": "` + previousSHA + `"}]`))
	}))
	defer server.Close()

	client := NewClient("test-token", server.URL)

	sha, err := client.GetLastSuccessfulPipelineSHA("123", "develop", currentSHA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sha != previousSHA {
		t.Fatalf("expected %q, got %q", previousSHA, sha)
	}
}
