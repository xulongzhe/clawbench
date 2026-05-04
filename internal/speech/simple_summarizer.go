package speech

import "context"

// SimpleSummarizer performs no AI-based summarization.
// It only strips markdown formatting and truncates long text,
// making it a zero-cost, zero-latency summarizer suitable for
// cases where raw cleaned text is acceptable for TTS.
type SimpleSummarizer struct{}

// NewSimpleSummarizer creates a SimpleSummarizer.
func NewSimpleSummarizer() *SimpleSummarizer {
	return &SimpleSummarizer{}
}

// Summarize strips markdown and truncates text without calling any AI model.
// Uses SimpleMaxSummarizeRunes (1000) as the truncation limit instead of
// the AI summarizer's DefaultMaxSummarizeRunes (10000), because without
// AI condensation, longer text would be too verbose for TTS.
// The language parameter is ignored — simple summarizer has no language awareness.
func (s *SimpleSummarizer) Summarize(_ context.Context, text string, _ string) (string, error) {
	cleaned := StripMarkdown(text)

	runes := []rune(cleaned)
	if len(runes) > SimpleMaxSummarizeRunes {
		cleaned = string(runes[len(runes)-SimpleMaxSummarizeRunes:])
	}

	return cleaned, nil
}
