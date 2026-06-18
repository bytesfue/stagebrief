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

func (c *Client) PostSummary(projectName, summary string, commits []gitlab.Commit, files []gitlab.FileDiff) error {
	msg := buildMessage(projectName, summary, commits, files)

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

func buildMessage(projectName, summary string, commits []gitlab.Commit, files []gitlab.FileDiff) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("🚀 *Staging updated — %s*\n\n", projectName))

	sb.WriteString(summary)
	sb.WriteString("\n\n")

	sb.WriteString("*Commits:*\n")
	if len(commits) == 0 {
		sb.WriteString("  (none)\n")
	}
	for _, c := range commits {
		sb.WriteString(fmt.Sprintf("  • `%s` %s\n", c.ID[:8], c.Title))
	}

	sb.WriteString("\n*Changed files:*\n")
	if len(files) == 0 {
		sb.WriteString("  (none)\n")
	}
	for _, f := range files {
		sb.WriteString(fmt.Sprintf("  • %s %s\n", fileStatus(f), f.NewPath))
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
