package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/bytesfue/stagingbrief/internal/gitlab"
)

type payload struct {
	Channel string `json:"channel"`
	Text    string `json:"text"`
}

// maxMessageLength is Slack's documented limit for the chat.postMessage
// text field (40,000 characters). We treat it as a byte cap, which is
// strictly more conservative than a character cap for any multi-byte
// (e.g. emoji-containing) content. Var rather than const so tests can
// shrink it to exercise truncation without building 40,000-byte fixtures.
var maxMessageLength = 40000

const truncationNotice = "\n\n_⚠️ Message truncated — it exceeded Slack's size limit. See GitLab for the full commit and file list._"

// truncateForSlack hard-truncates msg to fit within Slack's message size
// limit, appending a notice so the truncation is visible rather than
// silently cutting off content (or failing to post at all).
func truncateForSlack(msg string) string {
	if len(msg) <= maxMessageLength {
		return msg
	}

	cut := maxMessageLength - len(truncationNotice)
	if cut < 0 {
		cut = 0
	}
	truncated := msg[:cut]

	// Avoid splitting a multi-byte UTF-8 rune (e.g. an emoji) at the cut point.
	for len(truncated) > 0 {
		r, size := utf8.DecodeLastRuneInString(truncated)
		if r != utf8.RuneError || size != 1 {
			break
		}
		truncated = truncated[:len(truncated)-1]
	}

	return truncated + truncationNotice
}

func (c *Client) PostSummary(
	projectName,
	summary string,
	commits []gitlab.Commit,
	files []gitlab.FileDiff,
	cfg MessageConfig,
) error {
	msg := truncateForSlack(buildMessage(projectName, summary, commits, files, cfg))

	body, err := json.Marshal(payload{
		Channel: c.channel,
		Text:    msg,
	})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL, bytes.NewReader(body))
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

	sb.WriteString(fmt.Sprintf("🚀 *Staging updated — %s*\n\n", escapeSlack(projectName)))

	sb.WriteString(escapeSlack(summary))
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
			sb.WriteString(fmt.Sprintf("  • `%s` %s\n", shortSHA(c.ID), escapeSlack(c.Title)))
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
			sb.WriteString(fmt.Sprintf("  • %s %s\n", f.Status(), escapeSlack(f.NewPath)))
		}
		if len(files) > len(shown) {
			sb.WriteString(fmt.Sprintf("  _... and %d more files_\n", len(files)-len(shown)))
		}
	}

	sb.WriteString("\n_⚠️ AI-generated summary — may contain mistakes. Always check the raw commits above._")

	return sb.String()
}

// escapeSlack escapes text per Slack's mrkdwn rules so it can't be
// interpreted as markup or special mentions (e.g. <!channel>, <@U…>).
// See https://api.slack.com/reference/surfaces/formatting#escaping
func escapeSlack(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// shortSHA returns up to the first 8 characters of a commit SHA, without
// panicking if the SHA is shorter than that (e.g. in tests or truncated data).
func shortSHA(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
