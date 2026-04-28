package speech

import (
	"context"
	"regexp"
	"strings"
)

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

// Pre-compiled regexes for StripMarkdown.
var (
	reCodeBlock      = regexp.MustCompile("(?s)```.*?```")
	reInlineCode     = regexp.MustCompile("`[^`]+`")
	reBoldAsterisk   = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	reBoldUnderscore = regexp.MustCompile(`__([^_]+)__`)
	reItalicAsterisk = regexp.MustCompile(`\*([^*]+)\*`)
	reItalicUnder    = regexp.MustCompile(`_([^_]+)_`)
	reHeaders        = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	reLinks          = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	reImages         = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`)
	reHorizontalRule = regexp.MustCompile(`(?m)^[-*_]{3,}\s*$`)
	reMultiBlank     = regexp.MustCompile(`\n{3,}`)
)

// StripMarkdown removes common markdown formatting from text.
// Should be called on LLM output before passing to TTS synthesis.
func StripMarkdown(text string) string {
	text = reCodeBlock.ReplaceAllString(text, "")
	text = reInlineCode.ReplaceAllString(text, "")
	text = reBoldAsterisk.ReplaceAllString(text, "$1")
	text = reBoldUnderscore.ReplaceAllString(text, "$1")
	text = reItalicAsterisk.ReplaceAllString(text, "$1")
	text = reItalicUnder.ReplaceAllString(text, "$1")
	text = reHeaders.ReplaceAllString(text, "")
	text = reLinks.ReplaceAllString(text, "$1")
	text = reImages.ReplaceAllString(text, "")
	text = reHorizontalRule.ReplaceAllString(text, "")
	text = reMultiBlank.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}
