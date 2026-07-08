package slack

import (
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
