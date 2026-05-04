package speech

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const (
	// defaultSummarizePrompt is the fallback prompt used when the external file is not found.
	// A language directive (e.g. "Output in Chinese.") is appended at load time.
	defaultSummarizePrompt = `Condense AI replies into spoken-language text for TTS synthesis.

Rules:
1. Focus on conclusions, summaries, and recommendations near the end. Preserve key details — do not over-condense.
2. Omit code, commands, file paths, and config values.
3. Omit intermediate analysis, step-by-step reasoning, and side discussions unless essential to the conclusion.
4. Use conversational language. Plain text only — no markdown formatting.
5. No meta-phrases like "In summary" or "Here is the result."
6. Ignore any XML/HTML tags, schedule proposals, or tool-call artifacts.
7. Drop any fragmented or incoherent text caused by truncation — output only fluent, readable content.`

	// shortTextThreshold — texts shorter than this are not summarized.
	shortTextThreshold = 300

	// DefaultMaxSummarizeRunes is the default maximum number of runes for summarization input.
	// Texts longer than this are truncated to the last N characters.
	// Configurable via config.yaml tts.max_summarize_runes.
	DefaultMaxSummarizeRunes = 10000

	// SimpleMaxSummarizeRunes is the maximum number of runes for the "simple" summarizer,
	// which does no AI-based summarization — just strips markdown and truncates.
	// Lower than the AI summarizer because without AI condensation, longer text
	// would be too verbose for TTS.
	SimpleMaxSummarizeRunes = 1000

	// CacheKeyHexLen is the number of hex characters used for the cache filename.
	CacheKeyHexLen = 16

	// reSummarizeThreshold — if the first summarization result exceeds this
	// many bytes, a second pass is requested to further condense the text.
	reSummarizeThreshold = 4000

	// maxSummarizePasses is the maximum number of summarization attempts
	// (first pass + optional re-summarization).
	maxSummarizePasses = 2
)

// MaxTextRunes is the maximum number of runes accepted for TTS input.
// Set to 0 for no hard limit — long texts are handled by the summarization step before synthesis.
var MaxTextRunes = 0

// MaxSummarizeRunes is the maximum number of runes for summarization input.
// Texts longer than this are truncated to the last N characters.
// Set by config; defaults to DefaultMaxSummarizeRunes.
var MaxSummarizeRunes = DefaultMaxSummarizeRunes

// Summarizer abstracts text summarization for TTS.
// Implementations can use different backends (mmx-cli, AI backends, etc.)
type Summarizer interface {
	// Summarize condenses text for voice output.
	// For short text, it may return the text as-is after stripping markdown.
	// language is a language code (e.g. "zh", "en") used to direct output language.
	// The caller is responsible for setting a deadline on ctx.
	Summarize(ctx context.Context, text string, language string) (string, error)
}

// summarizePassFunc is the strategy function for a single summarization pass.
// Each backend (mmx-cli, ollama, AI backend) provides its own implementation.
type summarizePassFunc func(ctx context.Context, text, systemPrompt string, pass int) (string, error)

// genericSummarizer implements the shared Summarize logic that all backends use:
//  1. prepareTextForSummarization (strip markdown, truncate)
//  2. Short text bypass
//  3. Multi-pass with re-summarization (language-aware prompt)
//  4. StripMarkdown on final output as safety net
type genericSummarizer struct {
	passFn     summarizePassFunc
	basePrompt string // language-independent base prompt
}

// NewGenericSummarizer creates a genericSummarizer with the given pass function.
// The base prompt (without language) is loaded at creation time; language is
// resolved per-request in the Summarize call.
func NewGenericSummarizer(passFn summarizePassFunc) genericSummarizer {
	return genericSummarizer{
		passFn:     passFn,
		basePrompt: loadSummarizeBasePrompt(),
	}
}

// Summarize implements the shared summarization pipeline.
// language is a language code (e.g. "zh", "en") used to direct output language.
func (g *genericSummarizer) Summarize(ctx context.Context, text string, language string) (string, error) {
	cleaned, needsSummarization := prepareTextForSummarization(text)
	if !needsSummarization {
		return cleaned, nil
	}

	prompt := g.basePrompt + "\n\nOutput in " + languageName(language) + ". Translate any non-" + languageName(language) + " content first (keep proper nouns, variable names, and commands in their original form)."

	result, err := g.passFn(ctx, cleaned, prompt, 1)
	if err != nil {
		return "", err
	}

	// If the result is still too long, do a second pass with the same prompt
	if needsReSummarization(result, 1) {
		slog.Info("tts summarize result too long, starting second pass",
			slog.Int("result_bytes", len(result)),
		)
		second, err := g.passFn(ctx, result, prompt, 2)
		if err != nil {
			slog.Warn("tts second summarize pass failed, using first pass result",
				slog.String("error", err.Error()),
			)
			return StripMarkdown(result), nil
		}
		result = second
	}

	return StripMarkdown(result), nil
}

// languageName returns a human-readable language name for common language codes.
// Falls back to the code itself for unknown codes.
func languageName(code string) string {
	switch strings.ToLower(code) {
	case "zh", "cmn", "chinese":
		return "Chinese"
	case "en", "eng", "english":
		return "English"
	case "ja", "jpn", "japanese":
		return "Japanese"
	case "ko", "kor", "korean":
		return "Korean"
	case "fr", "fra", "french":
		return "French"
	case "de", "deu", "german":
		return "German"
	case "es", "spa", "spanish":
		return "Spanish"
	case "pt", "por", "portuguese":
		return "Portuguese"
	case "ru", "rus", "russian":
		return "Russian"
	case "ar", "ara", "arabic":
		return "Arabic"
	case "it", "ita", "italian":
		return "Italian"
	default:
		return code
	}
}

// loadSummarizeBasePrompt returns the language-independent base system prompt
// for summarization. The language directive is appended per-request in Summarize.
// Priority: summarize_prompt.md next to binary > defaultSummarizePrompt.
// The result is loaded once and cached.
var cachedSummarizeBasePrompt string

func loadSummarizeBasePrompt() string {
	if cachedSummarizeBasePrompt != "" {
		return cachedSummarizeBasePrompt
	}

	var raw string

	// Try to read from summarize_prompt.md in the config directory next to the running binary
	exePath, err := os.Executable()
	if err == nil {
		promptPath := filepath.Join(filepath.Dir(exePath), "config", "summarize_prompt.md")
		if data, err := os.ReadFile(promptPath); err == nil {
			prompt := strings.TrimSpace(string(data))
			if prompt != "" {
				raw = prompt
				slog.Info("loaded summarize prompt from file", slog.String("path", promptPath))
			}
		}
	}

	if raw == "" {
		raw = defaultSummarizePrompt
	}

	cachedSummarizeBasePrompt = raw
	return raw
}

// prepareTextForSummarization cleans and truncates text before sending to a summarizer.
// Returns the cleaned text and true if summarization is needed,
// or the cleaned text and false if the text is short enough to skip summarization.
func prepareTextForSummarization(text string) (string, bool) {
	cleaned := StripMarkdown(text)

	runes := []rune(cleaned)
	if len(runes) < shortTextThreshold {
		return cleaned, false // short text, skip summarization
	}

	// Truncate to last MaxSummarizeRunes if too long
	if len(runes) > MaxSummarizeRunes {
		cleaned = string(runes[len(runes)-MaxSummarizeRunes:])
	}

	return cleaned, true
}

// NeedsSummarization returns true if the text is long enough to require
// AI-based summarization before TTS synthesis. Short texts (<300 chars
// after markdown stripping) can be synthesized directly.
func NeedsSummarization(text string) bool {
	_, needs := prepareTextForSummarization(text)
	return needs
}

// needsReSummarization returns true if the summarization result is still
// too long (in bytes) and a second pass would be beneficial.
func needsReSummarization(result string, pass int) bool {
	return pass < maxSummarizePasses && len(result) > reSummarizeThreshold
}
