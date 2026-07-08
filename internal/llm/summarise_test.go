package llm

import (
	"strings"
	"testing"

	"github.com/bytesfue/stagingbrief/internal/gitlab"
)

func TestBuildPrompt(t *testing.T) {
	tests := []struct {
		name           string
		input          Input
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:  "empty commits and files",
			input: Input{},
			wantContains: []string{
				"Commits:\n  (none)\n",
				"Changed files:\n  (none)\n",
			},
		},
		{
			name: "populated commits and files",
			input: Input{
				Commits: []gitlab.Commit{
					{ID: "aaa111", Title: "Fix login bug"},
					{ID: "bbb222", Title: "Update checkout flow  "},
				},
				Files: []gitlab.FileDiff{
					{NewPath: "src/checkout/index.tsx", NewFile: true},
					{NewPath: "src/login/form.tsx", DeletedFile: true},
				},
			},
			wantContains: []string{
				"- Fix login bug\n",
				"- Update checkout flow\n",
				"A  src/checkout/index.tsx",
				"D  src/login/form.tsx",
			},
			wantNotContain: []string{
				"(none)",
			},
		},
		{
			name: "commits present, no files",
			input: Input{
				Commits: []gitlab.Commit{{ID: "aaa111", Title: "Bump dependency"}},
			},
			wantContains: []string{
				"- Bump dependency\n",
				"Changed files:\n  (none)\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildPrompt(tt.input)
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("BuildPrompt() missing %q in output:\n%s", want, got)
				}
			}
			for _, notWant := range tt.wantNotContain {
				if strings.Contains(got, notWant) {
					t.Errorf("BuildPrompt() unexpectedly contains %q in output:\n%s", notWant, got)
				}
			}
		})
	}
}

func TestFileStatus(t *testing.T) {
	tests := []struct {
		name string
		file gitlab.FileDiff
		want string
	}{
		{"added", gitlab.FileDiff{NewFile: true}, "A"},
		{"deleted", gitlab.FileDiff{DeletedFile: true}, "D"},
		{"renamed", gitlab.FileDiff{RenamedFile: true}, "R"},
		{"modified (default)", gitlab.FileDiff{}, "M"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fileStatus(tt.file); got != tt.want {
				t.Errorf("fileStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}
