package ai

// OpenCodeBackend implements AIBackend for OpenCode CLI
type OpenCodeBackend struct{}

// Name returns the backend identifier
func (o *OpenCodeBackend) Name() string {
	return "opencode"
}
