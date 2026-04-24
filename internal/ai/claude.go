package ai

// ClaudeBackend implements AIBackend for Claude CLI
type ClaudeBackend struct{}

// Name returns the backend identifier
func (c *ClaudeBackend) Name() string {
	return "claude"
}
