package speech

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- StripMarkdown tests ---

func TestStripMarkdown_CodeBlock(t *testing.T) {
	input := "Here is some code:\n```go\nfmt.Println(\"hello\")\n```\nAnd more text."
	result := StripMarkdown(input)
	assert.NotContains(t, result, "```")
	assert.NotContains(t, result, "fmt.Println")
	assert.Contains(t, result, "Here is some code")
	assert.Contains(t, result, "And more text")
}

func TestStripMarkdown_InlineCode(t *testing.T) {
	input := "Use the `fmt.Println` function to print."
	result := StripMarkdown(input)
	assert.NotContains(t, result, "`")
	assert.Contains(t, result, "Use the")
	assert.Contains(t, result, "function to print")
}

func TestStripMarkdown_Bold(t *testing.T) {
	input := "This is **bold** and __also bold__ text."
	result := StripMarkdown(input)
	assert.Equal(t, "This is bold and also bold text.", result)
}

func TestStripMarkdown_Italic(t *testing.T) {
	input := "This is *italic* and _also italic_ text."
	result := StripMarkdown(input)
	assert.Equal(t, "This is italic and also italic text.", result)
}

func TestStripMarkdown_Headers(t *testing.T) {
	input := "# Title\n## Subtitle\n### H3\nNormal text"
	result := StripMarkdown(input)
	assert.NotContains(t, result, "#")
	assert.Contains(t, result, "Title")
	assert.Contains(t, result, "Normal text")
}

func TestStripMarkdown_Links(t *testing.T) {
	input := "Visit [the website](https://example.com) for details."
	result := StripMarkdown(input)
	assert.NotContains(t, result, "https://")
	assert.NotContains(t, result, "(")
	assert.Contains(t, result, "Visit")
	assert.Contains(t, result, "the website")
	assert.Contains(t, result, "for details")
}

func TestStripMarkdown_Images(t *testing.T) {
	input := "Here is an image: ![alt text](image.png) and text after."
	result := StripMarkdown(input)
	assert.NotContains(t, result, "![]")
	assert.NotContains(t, result, "image.png")
	assert.Contains(t, result, "Here is an image")
	assert.Contains(t, result, "and text after")
}

func TestStripMarkdown_HorizontalRule(t *testing.T) {
	input := "Above\n---\nBelow"
	result := StripMarkdown(input)
	assert.NotContains(t, result, "---")
	assert.Contains(t, result, "Above")
	assert.Contains(t, result, "Below")
}

func TestStripMarkdown_MultipleBlankLines(t *testing.T) {
	input := "A\n\n\n\n\nB"
	result := StripMarkdown(input)
	assert.NotContains(t, result, "\n\n\n")
	assert.Contains(t, result, "A")
	assert.Contains(t, result, "B")
}

func TestStripMarkdown_PlainText(t *testing.T) {
	input := "Just plain text without any formatting."
	result := StripMarkdown(input)
	assert.Equal(t, input, result)
}

func TestStripMarkdown_EmptyString(t *testing.T) {
	result := StripMarkdown("")
	assert.Equal(t, "", result)
}

func TestStripMarkdown_ComplexMix(t *testing.T) {
	input := `# Project Setup

First, install **dependencies** using ` + "`npm install`" + `.

Then configure the [settings](/config):

` + "```json" + `
{
  "port": 3000
}
` + "```" + `

---

Run with *npm start*.`
	result := StripMarkdown(input)
	assert.NotContains(t, result, "#")
	assert.NotContains(t, result, "```")
	assert.NotContains(t, result, "`")
	assert.NotContains(t, result, "**")
	assert.NotContains(t, result, "http")
	assert.Contains(t, result, "Project Setup")
	assert.Contains(t, result, "dependencies")
	assert.Contains(t, result, "settings")
}

// --- NewMiniMaxProvider defaults ---

func TestNewMiniMaxProvider_Defaults(t *testing.T) {
	p := NewMiniMaxProvider()
	assert.Equal(t, "MiniMax-Text-02-HS", p.SummarizeModel)
	assert.Equal(t, "speech-2.8-hd", p.TTSModel)
	assert.Equal(t, "female-chengshu", p.TTSVoice)
	assert.Equal(t, "zh", p.TTSLanguage)
	assert.Equal(t, 1.5, p.TTSSpeed)
	assert.Equal(t, "mp3", p.TTSFormat)
}

// --- Summarize short text bypass ---

func TestSummarize_ShortText_BypassesLLM(t *testing.T) {
	p := NewMiniMaxProvider()
	shortText := "这是一个简短的消息，不需要总结。"
	result, err := p.Summarize(context.Background(), shortText)
	assert.NoError(t, err)
	// Short text should be returned as-is (after markdown stripping)
	assert.Contains(t, result, "简短的消息")
}

func TestSummarize_ShortTextWithMarkdown_StripsMarkdown(t *testing.T) {
	p := NewMiniMaxProvider()
	input := "Short **bold** and *italic* text."
	result, err := p.Summarize(context.Background(), input)
	assert.NoError(t, err)
	assert.NotContains(t, result, "**")
	assert.NotContains(t, result, "*")
	assert.Contains(t, result, "bold")
	assert.Contains(t, result, "italic")
}

// --- Summarize long text (requires mmx CLI, skip if unavailable) ---

func TestSummarize_LongText_WithCLI(t *testing.T) {
	if _, err := os.Stat("/usr/local/bin/mmx"); err != nil {
		// Check PATH for mmx
		if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".nvm/versions/node/v24.14.0/bin/mmx")); err != nil {
			t.Skip("mmx CLI not available, skipping integration test")
		}
	}

	p := NewMiniMaxProvider()
	longText := strings.Repeat("这是一个较长的AI回复内容，包含了详细的技术分析和代码示例。", 10)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	result, err := p.Summarize(ctx, longText)
	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	// Summary should be shorter than original
	assert.Less(t, len([]rune(result)), len([]rune(longText)))
}

// --- Summarize context cancellation ---

func TestSummarize_CancelledContext(t *testing.T) {
	p := NewMiniMaxProvider()
	longText := strings.Repeat("这是需要被总结的长文本内容。", 50)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := p.Summarize(ctx, longText)
	assert.Error(t, err)
}

// --- Synthesize integration test (requires mmx CLI) ---

func TestSynthesize_WithCLI(t *testing.T) {
	if _, err := os.Stat("/usr/local/bin/mmx"); err != nil {
		if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".nvm/versions/node/v24.14.0/bin/mmx")); err != nil {
			t.Skip("mmx CLI not available, skipping integration test")
		}
	}

	p := NewMiniMaxProvider()
	outputPath := filepath.Join(t.TempDir(), "test_output.mp3")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := p.Synthesize(ctx, "这是一个测试语音。", outputPath)
	assert.NoError(t, err)

	// Verify output file exists and has content
	info, err := os.Stat(outputPath)
	assert.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0))
}

// --- Synthesize creates output directory ---

func TestSynthesize_CreatesDirectory(t *testing.T) {
	if _, err := os.Stat("/usr/local/bin/mmx"); err != nil {
		if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".nvm/versions/node/v24.14.0/bin/mmx")); err != nil {
			t.Skip("mmx CLI not available, skipping integration test")
		}
	}

	p := NewMiniMaxProvider()
	nestedDir := filepath.Join(t.TempDir(), "deep", "nested", "dir")
	outputPath := filepath.Join(nestedDir, "output.mp3")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := p.Synthesize(ctx, "测试目录创建。", outputPath)
	assert.NoError(t, err)

	// Verify the directory was created
	_, err = os.Stat(nestedDir)
	assert.NoError(t, err)
}

// --- Synthesize context cancellation ---

func TestSynthesize_CancelledContext(t *testing.T) {
	p := NewMiniMaxProvider()
	outputPath := filepath.Join(t.TempDir(), "cancelled.mp3")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := p.Synthesize(ctx, "test", outputPath)
	assert.Error(t, err)
}

// --- Constants ---

func TestConstants(t *testing.T) {
	assert.Equal(t, 0, MaxTextRunes)
	assert.Equal(t, 16, CacheKeyHexLen)
}
