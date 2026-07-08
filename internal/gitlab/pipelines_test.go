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

func TestGetLastSuccessfulPipelineSHA_FindsPreviousOnSecondPage(t *testing.T) {
	const currentSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const previousSHA = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	var gotPages []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		gotPages = append(gotPages, page)

		w.Header().Set("Content-Type", "application/json")
		switch page {
		case "1":
			// Page 1 only has pipelines matching the current SHA — no previous found yet.
			w.Header().Set("X-Next-Page", "2")
			w.Write([]byte(`[{"id": 3, "sha": "` + currentSHA + `"}, {"id": 2, "sha": "` + currentSHA + `"}]`))
		case "2":
			w.Write([]byte(`[{"id": 1, "sha": "` + previousSHA + `"}]`))
		default:
			t.Fatalf("unexpected page requested: %q", page)
		}
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
	if len(gotPages) != 2 || gotPages[0] != "1" || gotPages[1] != "2" {
		t.Fatalf("expected pages [1 2] to be requested in order, got %v", gotPages)
	}
}

func TestGetLastSuccessfulPipelineSHA_NoPreviousAcrossMultiplePages(t *testing.T) {
	const currentSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")
		switch page {
		case "1":
			w.Header().Set("X-Next-Page", "2")
			w.Write([]byte(`[{"id": 2, "sha": "` + currentSHA + `"}]`))
		case "2":
			// Last page — still no pipeline with a different SHA.
			w.Write([]byte(`[{"id": 1, "sha": "` + currentSHA + `"}]`))
		default:
			t.Fatalf("unexpected page requested: %q", page)
		}
	}))
	defer server.Close()

	client := NewClient("test-token", server.URL)

	_, err := client.GetLastSuccessfulPipelineSHA("123", "develop", currentSHA)
	if !errors.Is(err, ErrNoPreviousPipeline) {
		t.Fatalf("expected ErrNoPreviousPipeline after exhausting all pages, got %v", err)
	}
}
