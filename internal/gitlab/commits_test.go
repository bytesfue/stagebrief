package gitlab

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestFileDiff_Status(t *testing.T) {
	tests := []struct {
		name string
		file FileDiff
		want string
	}{
		{"added", FileDiff{NewFile: true}, "A"},
		{"deleted", FileDiff{DeletedFile: true}, "D"},
		{"renamed", FileDiff{RenamedFile: true}, "R"},
		{"modified (default)", FileDiff{}, "M"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.file.Status(); got != tt.want {
				t.Errorf("Status() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetChangedFiles_UsesStraightComparison(t *testing.T) {
	var gotQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"commits": [], "diffs": []}`))
	}))
	defer server.Close()

	client := NewClient("test-token", server.URL)

	_, err := client.GetChangedFiles("123", "aaa111", "bbb222")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := gotQuery.Get("straight"); got != "true" {
		t.Fatalf("expected straight=true, got straight=%q (full query: %v)", got, gotQuery)
	}
	if got := gotQuery.Get("from"); got != "aaa111" {
		t.Fatalf("expected from=aaa111, got from=%q", got)
	}
	if got := gotQuery.Get("to"); got != "bbb222" {
		t.Fatalf("expected to=bbb222, got to=%q", got)
	}
}

func TestGetChangedFiles_ReturnsDiffs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"commits": [],
			"diffs": [
				{"old_path": "a.go", "new_path": "a.go", "new_file": false, "renamed_file": false, "deleted_file": false},
				{"old_path": "", "new_path": "b.go", "new_file": true, "renamed_file": false, "deleted_file": false}
			]
		}`))
	}))
	defer server.Close()

	client := NewClient("test-token", server.URL)

	diffs, err := client.GetChangedFiles("123", "aaa111", "bbb222")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(diffs) != 2 {
		t.Fatalf("expected 2 diffs, got %d", len(diffs))
	}
	if diffs[0].NewPath != "a.go" || diffs[0].NewFile {
		t.Errorf("unexpected first diff: %+v", diffs[0])
	}
	if diffs[1].NewPath != "b.go" || !diffs[1].NewFile {
		t.Errorf("unexpected second diff: %+v", diffs[1])
	}
}

func TestGetChangedFiles_PropagatesHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`server error`))
	}))
	defer server.Close()

	client := NewClient("test-token", server.URL)

	_, err := client.GetChangedFiles("123", "aaa111", "bbb222")
	if err == nil {
		t.Fatal("expected error when server returns 500, got nil")
	}
}

func TestGetCommitsBetween_UsesRefNameRange(t *testing.T) {
	var gotQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[
			{"id": "aaa111", "title": "First commit", "message": "First commit\n\nlonger body"},
			{"id": "bbb222", "title": "Second commit", "message": "Second commit"}
		]`))
	}))
	defer server.Close()

	client := NewClient("test-token", server.URL)

	commits, err := client.GetCommitsBetween("123", "aaa111", "bbb222")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := gotQuery.Get("ref_name"); got != "aaa111..bbb222" {
		t.Fatalf("expected ref_name=aaa111..bbb222, got %q", got)
	}

	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}
	if commits[0].ID != "aaa111" || commits[0].Title != "First commit" {
		t.Errorf("unexpected first commit: %+v", commits[0])
	}
	if commits[1].ID != "bbb222" || commits[1].Title != "Second commit" {
		t.Errorf("unexpected second commit: %+v", commits[1])
	}
}

func TestGetCommitsBetween_FollowsPagination(t *testing.T) {
	var gotPages []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		gotPages = append(gotPages, page)

		w.Header().Set("Content-Type", "application/json")
		switch page {
		case "1":
			w.Header().Set("X-Next-Page", "2")
			w.Write([]byte(`[
				{"id": "aaa111", "title": "First commit"},
				{"id": "bbb222", "title": "Second commit"}
			]`))
		case "2":
			// No X-Next-Page header set — signals this is the last page.
			w.Write([]byte(`[
				{"id": "ccc333", "title": "Third commit"}
			]`))
		default:
			t.Fatalf("unexpected page requested: %q", page)
		}
	}))
	defer server.Close()

	client := NewClient("test-token", server.URL)

	commits, err := client.GetCommitsBetween("123", "aaa111", "ddd444")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gotPages) != 2 || gotPages[0] != "1" || gotPages[1] != "2" {
		t.Fatalf("expected pages [1 2] to be requested in order, got %v", gotPages)
	}

	if len(commits) != 3 {
		t.Fatalf("expected 3 combined commits across both pages, got %d: %+v", len(commits), commits)
	}
	wantIDs := []string{"aaa111", "bbb222", "ccc333"}
	for i, want := range wantIDs {
		if commits[i].ID != want {
			t.Errorf("commits[%d].ID = %q, want %q", i, commits[i].ID, want)
		}
	}
}

func TestGetCommitsBetween_StopsAtMaxPagesCap(t *testing.T) {
	origMaxPages := maxPages
	maxPages = 2
	defer func() { maxPages = origMaxPages }()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		// Always claim there's a next page, to simulate a runaway/misbehaving server.
		w.Header().Set("X-Next-Page", "999")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id": "aaa111", "title": "commit"}]`))
	}))
	defer server.Close()

	client := NewClient("test-token", server.URL)

	_, err := client.GetCommitsBetween("123", "aaa111", "bbb222")
	if err == nil {
		t.Fatal("expected error when pagination exceeds maxPages cap, got nil")
	}
	if requests != maxPages {
		t.Errorf("expected exactly %d requests before giving up, got %d", maxPages, requests)
	}
}

func TestGetCommitsBetween_PropagatesHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`project not found`))
	}))
	defer server.Close()

	client := NewClient("test-token", server.URL)

	_, err := client.GetCommitsBetween("123", "aaa111", "bbb222")
	if err == nil {
		t.Fatal("expected error when server returns 404, got nil")
	}
}
