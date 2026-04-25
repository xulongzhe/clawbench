package ai

// GeminiBackend implements AIBackend for Gemini CLI
type GeminiBackend struct{}

// Name returns the backend identifier
func (g *GeminiBackend) Name() string {
	return "gemini"
}
