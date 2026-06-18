package slack

// MessageConfig controls what appears in the Slack message.
type MessageConfig struct {
	ShowChangedFiles bool
	ShowRawCommits   bool
	MaxFiles         int
	MaxCommits       int
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() MessageConfig {
	return MessageConfig{
		ShowChangedFiles: true,
		ShowRawCommits:   true,
		MaxFiles:         10,
		MaxCommits:       10,
	}
}
