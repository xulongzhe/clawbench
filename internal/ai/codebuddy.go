package ai

// CodebuddyBackend implements AIBackend for Codebuddy CLI
type CodebuddyBackend struct{}

// Name returns the backend identifier
func (c *CodebuddyBackend) Name() string {
	return "codebuddy"
}
