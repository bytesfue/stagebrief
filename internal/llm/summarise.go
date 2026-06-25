package llm

import (
	"fmt"
	"strings"

	"github.com/bytesfue/stagingbrief/internal/gitlab"
)

const systemPrompt = `You summarise code deployments for UX/UI designers and product
managers who need to know what to test on staging. You receive commit
messages and a list of changed file paths only — never source code.

Write:
1. 2-4 plain-language bullet points: what changed, what to look for visually
2. One line starting with "Worth testing if you care about:" naming the
   relevant areas (e.g. checkout, login, settings)
3. Optionally, one line flagging anything that may look broken but is
   intentional

Rules:
- No technical jargon, no function names, no internal file paths verbatim
- Translate file paths into plain area names (e.g. "src/components/checkout/"
  becomes "the checkout flow")
- Keep the summary under 120 words
- If nothing user-facing changed (e.g. only dependency updates, tests,
  CI config), say so clearly and briefly`

// Input is the data passed to the LLM for summarisation.
type Input struct {
	Commits []gitlab.Commit
	Files   []gitlab.FileDiff
}

// BuildPrompt formats commits and changed files into the user prompt.
func BuildPrompt(input Input) string {
	var sb strings.Builder

	sb.WriteString("Commits:\n")
	if len(input.Commits) == 0 {
		sb.WriteString("  (none)\n")
	}
	for _, c := range input.Commits {
		sb.WriteString(fmt.Sprintf("- %s\n", strings.TrimSpace(c.Title)))
	}

	sb.WriteString("\nChanged files:\n")
	if len(input.Files) == 0 {
		sb.WriteString("  (none)\n")
	}
	for _, f := range input.Files {
		sb.WriteString(fmt.Sprintf("  %s  %s\n", fileStatus(f), f.NewPath))
	}

	return sb.String()
}

func fileStatus(f gitlab.FileDiff) string {
	switch {
	case f.NewFile:
		return "A"
	case f.DeletedFile:
		return "D"
	case f.RenamedFile:
		return "R"
	default:
		return "M"
	}
}

// Summarise calls the LLM and returns a designer-friendly summary.
func Summarise(client *Client, input Input) (Result, error) {
	prompt := BuildPrompt(input)

	result, err := client.ChatCompletion(systemPrompt, prompt)
	if err != nil {
		return Result{}, fmt.Errorf("summarise: %w", err)
	}

	return result, nil
}
