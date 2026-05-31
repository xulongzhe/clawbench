package summarize

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Pre-compiled regexes for StripMarkdown.
var (
	reCodeBlock = regexp.MustCompile("(?s)```.*?```")
	// reAskQuestion matches <ask-question>...</ask-question> blocks (including
	// those wrapped inside markdown code fences like ```json...```).
	// The inner content is a JSON object with a "questions" array that must be
	// preserved for TTS summarization.
	reAskQuestion    = regexp.MustCompile("(?s)<ask-question>\\s*(```[a-z]*\\n)?(.*?)(```\\s*)?</ask-question>")
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
	// Extended markdown patterns for thorough TTS cleaning
	reStrikethrough   = regexp.MustCompile(`~~([^~]+)~~`)
	reBlockquote      = regexp.MustCompile(`(?m)^>\s?`)
	reUnorderedList   = regexp.MustCompile(`(?m)^[\s]*[-*+]\s+`)
	reOrderedList     = regexp.MustCompile(`(?m)^[\s]*\d+\.\s+`)
	reTaskList        = regexp.MustCompile(`(?m)^[\s]*[-*+]\s+\[[ xX]\]\s*`)
	reTablePipe       = regexp.MustCompile(`\|`)
	reTableDivider    = regexp.MustCompile(`(?m)^[\s|]*([-:]+[\s|:-]*)+$`)
	reHTMLTag         = regexp.MustCompile(`<[^>]+>`)
	reXMLTag          = regexp.MustCompile(`</?[a-zA-Z][^>]*>`)
	reAutolink        = regexp.MustCompile(`<([^>]+)>`)
	reFootnoteRef     = regexp.MustCompile(`\[\^[^\]]+\]`)
	reFootnoteDef     = regexp.MustCompile(`(?m)^\[\^[^\]]+\]:\s+.*$`)
	reEmojiShortcode  = regexp.MustCompile(`:[a-zA-Z0-9_+-]+:`)
	reBackslashEscape = regexp.MustCompile(`\\([\\` + "`" + `*_{}[\]()#+\-.!|~])`)
	// Angle-bracket URLs remaining after other stripping
	reBareURL = regexp.MustCompile(`https?://\S+`)
)

// InlineCodeMaxLen is the maximum content length (in runes) for inline code
// to be preserved (with backticks removed). Longer inline code is removed
// entirely — it typically contains code snippets not suitable for TTS.
// Configurable via config/config.yaml tts.inline_code_max_len.
var InlineCodeMaxLen = 100

// StripMarkdown removes common markdown formatting from text.
// Should be called on LLM output before passing to TTS synthesis.
func StripMarkdown(text string) string {
	// Phase 0: Resolve backslash escapes FIRST so that \* becomes *
	// and subsequent patterns can match the unescaped characters.
	text = reBackslashEscape.ReplaceAllString(text, "$1")

	// Phase 0.5: Preserve <ask-question> structured question content.
	// These contain JSON with questions/options that should be spoken aloud.
	// Extract the content before code-block stripping removes it.
	// Convert <ask-question>{"questions":[...]}</ask-question> into
	// a plain-text summary of the questions and options.
	text = reAskQuestion.ReplaceAllStringFunc(text, preserveAskQuestion)

	// Phase 1: Remove block-level elements
	text = reCodeBlock.ReplaceAllString(text, "")
	text = reFootnoteDef.ReplaceAllString(text, "")
	text = reTableDivider.ReplaceAllString(text, "")
	text = reHTMLTag.ReplaceAllString(text, "")
	text = reXMLTag.ReplaceAllString(text, "")

	// Phase 2: Remove inline formatting — task lists before unordered lists
	text = reTaskList.ReplaceAllString(text, "")
	text = reUnorderedList.ReplaceAllString(text, "")
	text = reOrderedList.ReplaceAllString(text, "")
	text = reBlockquote.ReplaceAllString(text, "")
	text = reStrikethrough.ReplaceAllString(text, "$1")
	text = stripInlineCode(text)
	text = reBoldAsterisk.ReplaceAllString(text, "$1")
	text = reBoldUnderscore.ReplaceAllString(text, "$1")
	text = reItalicAsterisk.ReplaceAllString(text, "$1")
	text = reItalicUnder.ReplaceAllString(text, "$1")
	text = reHeaders.ReplaceAllString(text, "")
	text = reLinks.ReplaceAllString(text, "$1")
	text = reAutolink.ReplaceAllString(text, "$1")
	text = reImages.ReplaceAllString(text, "")
	text = reHorizontalRule.ReplaceAllString(text, "")
	text = reFootnoteRef.ReplaceAllString(text, "")
	text = reEmojiShortcode.ReplaceAllString(text, "")

	// Phase 3: Remove table pipes (after content extraction)
	text = reTablePipe.ReplaceAllString(text, "")

	// Phase 4: Remove bare URLs (not useful for TTS)
	text = reBareURL.ReplaceAllString(text, "")

	// Phase 5: Clean up whitespace
	text = reMultiBlank.ReplaceAllString(text, "\n\n")

	// Final sweep: remove any remaining stray markdown punctuation that
	// survived the structured passes (loose *, #, ~, backticks, \, etc.)
	text = stripResidualMarkdown(text)

	return strings.TrimSpace(text)
}

// stripResidualMarkdown removes leftover markdown special characters that
// the regex passes above may have missed (e.g. orphaned *, #, ~, `, |, []).
// It preserves Chinese/English letters, digits, and readable punctuation.
var reResidualMarkdown = regexp.MustCompile(`[\\#*~` + "`" + `|]`)

func stripResidualMarkdown(text string) string {
	return reResidualMarkdown.ReplaceAllString(text, "")
}

// stripInlineCode processes inline code spans (`xxx`).
// Short content (≤ InlineCodeMaxLen runes) keeps its text — these are typically
// variable names, command names, or short terms worth reading aloud.
// Long content is removed entirely — these are typically code snippets.
func stripInlineCode(text string) string {
	return reInlineCode.ReplaceAllStringFunc(text, func(match string) string {
		// match includes the backticks; content is match[1:len-1]
		content := match[1 : len(match)-1]
		if len([]rune(content)) <= InlineCodeMaxLen {
			return content
		}
		return ""
	})
}

// askQuestionJSON is the JSON structure inside <ask-question> tags.
type askQuestionJSON struct {
	Questions []askQuestionItem `json:"questions"`
}

type askQuestionItem struct {
	Header      string           `json:"header"`
	Question    string           `json:"question"`
	Options     []askQuestionOpt `json:"options"`
	MultiSelect bool             `json:"multiSelect"`
}

type askQuestionOpt struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

// preserveAskQuestion converts a <ask-question>...</ask-question> block
// (whose JSON content may be wrapped in markdown code fences) into a
// plain-text summary suitable for TTS.  If the JSON cannot be parsed,
// the raw content is returned as-is so that the summarizer can still see it.
func preserveAskQuestion(match string) string {
	// Extract group(2) = the JSON content (between optional ```lang and optional ```)
	sub := reAskQuestion.FindStringSubmatch(match)
	if len(sub) < 3 {
		return match // no useful capture, return as-is
	}
	jsonText := strings.TrimSpace(sub[2])

	var data askQuestionJSON
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		// Not valid JSON — return the raw text so the summarizer can still read it
		return jsonText
	}

	var b strings.Builder
	for i, q := range data.Questions {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(q.Question)
		if q.Header != "" {
			fmt.Fprintf(&b, " (%s)", q.Header)
		}
		if len(q.Options) > 0 {
			b.WriteString(": ")
			for j, o := range q.Options {
				if j > 0 {
					b.WriteString(", ")
				}
				b.WriteString(o.Label)
				if o.Description != "" && o.Description != o.Label {
					fmt.Fprintf(&b, " — %s", o.Description)
				}
			}
		}
	}
	return b.String()
}
