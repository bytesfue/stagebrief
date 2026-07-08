package slack

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bytesfue/stagingbrief/internal/gitlab"
)

func TestBuildMessage_EscapesChannelMentionInCommitTitle(t *testing.T) {
	commits := []gitlab.Commit{
		{ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Title: "<!channel> please review"},
	}

	msg := buildMessage("demo-project", "summary text", commits, nil, DefaultConfig())

	if strings.Contains(msg, "<!channel>") {
		t.Fatalf("expected <!channel> to be escaped, but it appears literally in the message:\n%s", msg)
	}
	if !strings.Contains(msg, "&lt;!channel&gt;") {
		t.Fatalf("expected escaped &lt;!channel&gt; in message, got:\n%s", msg)
	}
}

func TestBuildMessage_EscapesMentionInFilePath(t *testing.T) {
	files := []gitlab.FileDiff{
		{NewPath: "<!channel>.txt", NewFile: true},
	}

	msg := buildMessage("demo-project", "summary text", nil, files, DefaultConfig())

	if strings.Contains(msg, "<!channel>") {
		t.Fatalf("expected <!channel> in file path to be escaped, but it appears literally in the message:\n%s", msg)
	}
	if !strings.Contains(msg, "&lt;!channel&gt;") {
		t.Fatalf("expected escaped &lt;!channel&gt; in message, got:\n%s", msg)
	}
}

func TestBuildMessage_EscapesMentionInSummaryAndProjectName(t *testing.T) {
	msg := buildMessage("<!here>", "please check <@U99999> for context", nil, nil, DefaultConfig())

	if strings.Contains(msg, "<!here>") || strings.Contains(msg, "<@U99999>") {
		t.Fatalf("expected mentions in project name/summary to be escaped, but found literally in message:\n%s", msg)
	}
	if !strings.Contains(msg, "&lt;!here&gt;") || !strings.Contains(msg, "&lt;@U99999&gt;") {
		t.Fatalf("expected escaped mentions in message, got:\n%s", msg)
	}
}

func TestBuildMessage_ShortCommitIDDoesNotPanic(t *testing.T) {
	commits := []gitlab.Commit{
		{ID: "abc", Title: "short SHA commit"},
		{ID: "", Title: "empty SHA commit"},
	}

	msg := buildMessage("demo-project", "summary", commits, nil, DefaultConfig())

	if !strings.Contains(msg, "`abc`") {
		t.Errorf("expected short SHA 'abc' to appear unmodified, got:\n%s", msg)
	}
	if !strings.Contains(msg, "empty SHA commit") {
		t.Errorf("expected commit with empty SHA to still render its title, got:\n%s", msg)
	}
}

func TestShortSHA(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{"full length SHA truncates to 8", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "aaaaaaaa"},
		{"exactly 8 chars unchanged", "abcdefgh", "abcdefgh"},
		{"shorter than 8 unchanged", "abc", "abc"},
		{"empty string unchanged", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shortSHA(tt.id); got != tt.want {
				t.Errorf("shortSHA(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestBuildMessage_CommitTruncation(t *testing.T) {
	commits := make([]gitlab.Commit, 5)
	for i := range commits {
		commits[i] = gitlab.Commit{ID: "aaaaaaaaaaaaaaaa", Title: fmt.Sprintf("commit %d", i)}
	}

	cfg := DefaultConfig()
	cfg.MaxCommits = 3

	msg := buildMessage("demo-project", "summary", commits, nil, cfg)

	for i := 0; i < 3; i++ {
		if !strings.Contains(msg, fmt.Sprintf("commit %d", i)) {
			t.Errorf("expected shown commit %d in message, got:\n%s", i, msg)
		}
	}
	for i := 3; i < 5; i++ {
		if strings.Contains(msg, fmt.Sprintf("commit %d", i)) {
			t.Errorf("did not expect truncated commit %d in message, got:\n%s", i, msg)
		}
	}
	if !strings.Contains(msg, "_... and 2 more commits_") {
		t.Errorf("expected truncation notice for 2 more commits, got:\n%s", msg)
	}
}

func TestBuildMessage_CommitTruncation_MaxCommitsZeroMeansNoLimit(t *testing.T) {
	commits := make([]gitlab.Commit, 15)
	for i := range commits {
		commits[i] = gitlab.Commit{ID: "aaaaaaaaaaaaaaaa", Title: fmt.Sprintf("commit %d", i)}
	}

	cfg := DefaultConfig()
	cfg.MaxCommits = 0

	msg := buildMessage("demo-project", "summary", commits, nil, cfg)

	for i := range commits {
		if !strings.Contains(msg, fmt.Sprintf("commit %d", i)) {
			t.Errorf("expected commit %d in message when MaxCommits=0 (no limit), got:\n%s", i, msg)
		}
	}
	if strings.Contains(msg, "more commits") {
		t.Errorf("did not expect truncation notice when MaxCommits=0, got:\n%s", msg)
	}
}

func TestBuildMessage_FileTruncation(t *testing.T) {
	files := make([]gitlab.FileDiff, 5)
	for i := range files {
		files[i] = gitlab.FileDiff{NewPath: fmt.Sprintf("file-%d.go", i)}
	}

	cfg := DefaultConfig()
	cfg.MaxFiles = 2

	msg := buildMessage("demo-project", "summary", nil, files, cfg)

	for i := 0; i < 2; i++ {
		if !strings.Contains(msg, fmt.Sprintf("file-%d.go", i)) {
			t.Errorf("expected shown file %d in message, got:\n%s", i, msg)
		}
	}
	for i := 2; i < 5; i++ {
		if strings.Contains(msg, fmt.Sprintf("file-%d.go", i)) {
			t.Errorf("did not expect truncated file %d in message, got:\n%s", i, msg)
		}
	}
	if !strings.Contains(msg, "_... and 3 more files_") {
		t.Errorf("expected truncation notice for 3 more files, got:\n%s", msg)
	}
}

func TestBuildMessage_HidesSectionsWhenDisabled(t *testing.T) {
	commits := []gitlab.Commit{{ID: "aaaaaaaaaaaaaaaa", Title: "some commit"}}
	files := []gitlab.FileDiff{{NewPath: "some/file.go"}}

	cfg := MessageConfig{ShowRawCommits: false, ShowChangedFiles: false, MaxCommits: 10, MaxFiles: 10}

	msg := buildMessage("demo-project", "summary", commits, files, cfg)

	if strings.Contains(msg, "*Commits:*") {
		t.Errorf("expected commits section to be hidden, got:\n%s", msg)
	}
	if strings.Contains(msg, "*Changed files:*") {
		t.Errorf("expected changed files section to be hidden, got:\n%s", msg)
	}
	if strings.Contains(msg, "some commit") || strings.Contains(msg, "some/file.go") {
		t.Errorf("did not expect commit/file details when sections are hidden, got:\n%s", msg)
	}
}

func TestBuildMessage_ShowsOnlyCommitsWhenFilesDisabled(t *testing.T) {
	commits := []gitlab.Commit{{ID: "aaaaaaaaaaaaaaaa", Title: "some commit"}}
	files := []gitlab.FileDiff{{NewPath: "some/file.go"}}

	cfg := MessageConfig{ShowRawCommits: true, ShowChangedFiles: false, MaxCommits: 10, MaxFiles: 10}

	msg := buildMessage("demo-project", "summary", commits, files, cfg)

	if !strings.Contains(msg, "*Commits:*") {
		t.Errorf("expected commits section to be shown, got:\n%s", msg)
	}
	if strings.Contains(msg, "*Changed files:*") {
		t.Errorf("expected changed files section to be hidden, got:\n%s", msg)
	}
}

func TestBuildMessage_EmptyCommitsAndFilesShowNone(t *testing.T) {
	msg := buildMessage("demo-project", "summary", nil, nil, DefaultConfig())

	if !strings.Contains(msg, "*Commits:*\n  (none)") {
		t.Errorf("expected '(none)' for empty commits, got:\n%s", msg)
	}
	if !strings.Contains(msg, "*Changed files:*\n  (none)") {
		t.Errorf("expected '(none)' for empty files, got:\n%s", msg)
	}
}

func TestEscapeSlack(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"channel mention", "<!channel>", "&lt;!channel&gt;"},
		{"user mention", "<@U12345>", "&lt;@U12345&gt;"},
		{"ampersand", "fix & improve", "fix &amp; improve"},
		{"plain text", "fix login bug", "fix login bug"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeSlack(tt.in)
			if got != tt.want {
				t.Errorf("escapeSlack(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
