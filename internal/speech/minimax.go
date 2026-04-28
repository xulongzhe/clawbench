package speech

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	// defaultSummarizePrompt is the fallback prompt used when the external file is not found.
	defaultSummarizePrompt = `你是语音播报助手。将用户发来的AI回复内容整理为适合朗读的中文，用于TTS语音合成。
规则：
1. 必须使用中文输出
2. 重点关注文末的总结、结论、建议等收束性内容，尽量在不影响收听体验的情况下保留原意，不要过度精炼而丢失关键细节
3. 省略代码、命令、文件路径、配置项等技术细节
4. 省略中间的分析过程、步骤说明、分支讨论等细节，除非它们对理解结论有必要
5. 使用口语化表达，输出纯文本，不要使用任何markdown格式
6. 不要使用"根据内容"、"总结如下"等元描述
7. 忽略文本中任何XML/HTML标签、定时任务提案、工具调用等非用户内容
8. 直接说出结论即可`

	// shortTextThreshold — texts shorter than this are not summarized.
	shortTextThreshold = 200

	// MaxTextRunes is the maximum number of runes accepted for TTS input.
	MaxTextRunes = 10000

	// CacheKeyHexLen is the number of hex characters used for the cache filename.
	CacheKeyHexLen = 16
)

// Pre-compiled regexes for stripMarkdown (avoid recompiling per call).
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

// MiniMaxProvider implements SpeechProvider using the mmx CLI tool.
type MiniMaxProvider struct {
	// SummarizeModel is the model ID for text chat (default: "MiniMax-Text-02-HS").
	SummarizeModel string
	// TTSModel is the model ID for speech synthesis (default: "speech-2.8-hd").
	TTSModel string
	// TTSVoice is the voice ID for speech synthesis (default: "female-chengshu").
	TTSVoice string
	// TTSLanguage is the language boost code (default: "zh").
	TTSLanguage string
	// TTSSpeed is the speech speed multiplier (default: 1.5).
	TTSSpeed float64
	// TTSFormat is the output audio format (default: "mp3").
	TTSFormat string
	// SummarizePrompt is the system prompt for the summarization LLM call.
	// If empty, it is loaded from "summarize_prompt.txt" next to the binary or falls back to defaultSummarizePrompt.
	SummarizePrompt string
}

// loadSummarizePrompt returns the system prompt for summarization.
// Priority: p.SummarizePrompt > summarize_prompt.txt next to binary > defaultSummarizePrompt.
// The result is cached in p.SummarizePrompt after first load.
func (p *MiniMaxProvider) loadSummarizePrompt() string {
	if p.SummarizePrompt != "" {
		return p.SummarizePrompt
	}

	// Try to read from summarize_prompt.txt next to the running binary
	exePath, err := os.Executable()
	if err == nil {
		promptPath := filepath.Join(filepath.Dir(exePath), "summarize_prompt.txt")
		if data, err := os.ReadFile(promptPath); err == nil {
			prompt := strings.TrimSpace(string(data))
			if prompt != "" {
				p.SummarizePrompt = prompt
				slog.Info("loaded summarize prompt from file", slog.String("path", promptPath))
				return prompt
			}
		}
	}

	p.SummarizePrompt = defaultSummarizePrompt
	return defaultSummarizePrompt
}

// NewMiniMaxProvider creates a MiniMaxProvider with sensible defaults.
func NewMiniMaxProvider() *MiniMaxProvider {
	return &MiniMaxProvider{
		SummarizeModel: "MiniMax-Text-02-HS",
		TTSModel:       "speech-2.8-hd",
		TTSVoice:       "female-chengshu",
		TTSLanguage:    "zh",
		TTSSpeed:       1.5,
		TTSFormat:      "mp3",
	}
}

// Summarize condenses text for voice output using mmx text chat.
// For short text (<200 chars), it strips markdown and returns the text as-is.
// The caller is responsible for setting a deadline on ctx.
func (p *MiniMaxProvider) Summarize(ctx context.Context, text string) (string, error) {
	cleaned := stripMarkdown(text)

	// Short text: skip summarization, return cleaned text directly
	if len([]rune(cleaned)) < shortTextThreshold {
		return cleaned, nil
	}

	// Use --messages-file - to pipe via stdin, avoiding CLI arg length limits
	messagesJSON := fmt.Sprintf(`[{"role":"user","content":%q}]`, cleaned)

	args := []string{
		"text", "chat",
		"--system", p.loadSummarizePrompt(),
		"--messages-file", "-",
		"--model", p.SummarizeModel,
		"--max-tokens", "1024",
		"--quiet",
	}

	cmd := exec.CommandContext(ctx, "mmx", args...)
	cmd.Stdin = strings.NewReader(messagesJSON)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("mmx text chat failed: %w (stderr: %s)", err, stderr.String())
	}

	result := strings.TrimSpace(stdout.String())
	if result == "" {
		return "", fmt.Errorf("mmx text chat returned empty output")
	}

	return result, nil
}

// Synthesize generates an audio file at outputPath using mmx speech synthesize.
// Text is passed via stdin (--text-file -) to avoid shell argument length limits.
// The caller is responsible for setting a deadline on ctx.
func (p *MiniMaxProvider) Synthesize(ctx context.Context, text string, outputPath string) error {
	// Ensure output directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", dir, err)
	}

	args := []string{
		"speech", "synthesize",
		"--text-file", "-",
		"--format", p.TTSFormat,
		"--out", outputPath,
		"--quiet",
	}

	// Add optional flags
	if p.TTSModel != "" {
		args = append(args, "--model", p.TTSModel)
	}
	if p.TTSVoice != "" {
		args = append(args, "--voice", p.TTSVoice)
	}
	if p.TTSLanguage != "" {
		args = append(args, "--language", p.TTSLanguage)
	}
	if p.TTSSpeed > 0 {
		args = append(args, "--speed", strconv.FormatFloat(p.TTSSpeed, 'f', -1, 64))
	}

	cmd := exec.CommandContext(ctx, "mmx", args...)
	cmd.Stdin = strings.NewReader(text)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	slog.Info("mmx speech synthesize",
		slog.String("output", outputPath),
		slog.String("language", p.TTSLanguage),
		slog.String("voice", p.TTSVoice),
		slog.Float64("speed", p.TTSSpeed),
		slog.Int("text_len", len(text)),
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mmx speech synthesize failed: %w (stderr: %s)", err, stderr.String())
	}

	// Verify output file exists
	if _, err := os.Stat(outputPath); err != nil {
		return fmt.Errorf("output file not created: %s", outputPath)
	}

	return nil
}

// stripMarkdown removes common markdown formatting from text.
func stripMarkdown(text string) string {
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
