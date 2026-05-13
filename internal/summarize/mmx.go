package summarize

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// MMXSummarizer implements Summarizer using the mmx CLI tool (mmx text chat).
// This is the "mmx-cli" backend in config — the default and fallback summarizer.
type MMXSummarizer struct {
	// Model is the model ID for text chat (default: "MiniMax-M2.7").
	Model string
	gs    ttsPipeline
}

// NewMMX creates an MMXSummarizer with sensible defaults.
func NewMMX() *MMXSummarizer {
	s := &MMXSummarizer{
		Model: "MiniMax-M2.7",
	}
	s.gs = NewTTSPipeline(s.DoSummarizePass)
	return s
}

// Summarize condenses text for voice output using mmx text chat.
func (s *MMXSummarizer) Summarize(ctx context.Context, text string, language string) (string, error) {
	return s.gs.Summarize(ctx, text, language)
}

// DoSummarizePass performs a single summarization pass using mmx text chat.
func (s *MMXSummarizer) DoSummarizePass(ctx context.Context, text, systemPrompt string, pass int) (string, error) {
	messagesJSON := fmt.Sprintf(`[{"role":"user","content":%q}]`, text)

	args := []string{
		"text", "chat",
		"--system", systemPrompt,
		"--messages-file", "-",
		"--model", s.Model,
		"--max-tokens", "1024",
		"--quiet",
	}

	cmd := exec.CommandContext(ctx, "mmx", args...)
	cmd.Stdin = strings.NewReader(messagesJSON)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("mmx text chat (pass %d) failed: %w (stderr: %s)", pass, err, stderr.String())
	}

	result := strings.TrimSpace(stdout.String())
	if result == "" {
		return "", fmt.Errorf("mmx text chat (pass %d) returned empty output", pass)
	}

	slog.Info("tts summarize pass completed",
		slog.Int("pass", pass),
		slog.Int("result_len", len([]rune(result))),
	)

	return result, nil
}
