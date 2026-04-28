package speech

import "context"

// SpeechProvider abstracts text summarization and audio synthesis.
// Implementations can be swapped (MiniMax, OpenAI, Azure, local TTS, etc.)
type SpeechProvider interface {
	// Summarize condenses text for voice output (3-5 sentences, no markdown, spoken language).
	// Returns the summary text or an error.
	Summarize(ctx context.Context, text string) (string, error)

	// Synthesize generates an audio file at outputPath from the given text.
	// Returns an error if synthesis fails.
	Synthesize(ctx context.Context, text string, outputPath string) error
}
