package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bytesfue/stagingbrief/internal/gitlab"
)

type payload struct {
	Channel string `json:"channel"`
	Text    string `json:"text"`
}

func (c *Client) PostSummary(
	projectName,
	summary string,
	commits []gitlab.Commit,
	files []gitlab.FileDiff,
	cfg MessageConfig,
) error {
	msg := buildMessage(projectName, summary, commits, files, cfg)

	body, err := json.Marshal(payload{
		Channel: c.channel,
		Text:    msg,
	})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.botToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("post to slack: %w", err)
	}
	defer resp.Body.Close()

	// Slack always returns 200 — errors are in the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if !apiResp.OK {
		return fmt.Errorf("slack API error: %s", apiResp.Error)
	}

	return nil
}

func buildMessage(
	projectName,
	summary string,
	commits []gitlab.Commit,
	files []gitlab.FileDiff,
	cfg MessageConfig,
) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("🚀 *Staging updated — %s*\n\n", projectName))

	sb.WriteString(summary)
	sb.WriteString("\n\n")

	if cfg.ShowRawCommits {
		sb.WriteString("\n*Commits:*\n")
		if len(commits) == 0 {
			sb.WriteString("  (none)\n")
		}
		shown := commits
		if cfg.MaxCommits > 0 && len(commits) > cfg.MaxCommits {
			shown = commits[:cfg.MaxCommits]
		}
		for _, c := range shown {
			sb.WriteString(fmt.Sprintf("  • `%s` %s\n", c.ID[:8], c.Title))
		}
		if len(commits) > len(shown) {
			sb.WriteString(fmt.Sprintf("  _... and %d more commits_\n", len(commits)-len(shown)))
		}
	}

	if cfg.ShowChangedFiles {
		sb.WriteString("\n*Changed files:*\n")
		if len(files) == 0 {
			sb.WriteString("  (none)\n")
		}
		shown := files
		if cfg.MaxFiles > 0 && len(files) > cfg.MaxFiles {
			shown = files[:cfg.MaxFiles]
		}
		for _, f := range shown {
			sb.WriteString(fmt.Sprintf("  • %s %s\n", fileStatus(f), f.NewPath))
		}
		if len(files) > len(shown) {
			sb.WriteString(fmt.Sprintf("  _... and %d more files_\n", len(files)-len(shown)))
		}
	}

	sb.WriteString("\n_⚠️ AI-generated summary — may contain mistakes. Always check the raw commits above._")

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
