package speech

import "context"

// SpeechProvider abstracts audio synthesis.
// Implementations can be swapped (Edge TTS, Piper, Kokoro, MOSS-Nano, etc.)
type SpeechProvider interface {
	// Synthesize generates an audio file at outputPath from the given text.
	// language is a language code (e.g. "zh", "en") — implementations that
	// support language-specific synthesis should use it; others may ignore it.
	// Returns an error if synthesis fails.
	Synthesize(ctx context.Context, text string, outputPath string, language string) error
}
