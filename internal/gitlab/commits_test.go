package gitlab

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

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
