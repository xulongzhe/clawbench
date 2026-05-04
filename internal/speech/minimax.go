package speech

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// MiniMaxProvider implements SpeechProvider using the mmx CLI tool.
type MiniMaxProvider struct {
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
}

// NewMiniMaxProvider creates a MiniMaxProvider with sensible defaults.
func NewMiniMaxProvider() *MiniMaxProvider {
	return &MiniMaxProvider{
		TTSModel:    "speech-2.8-hd",
		TTSVoice:    "female-chengshu",
		TTSLanguage: "zh",
		TTSSpeed:    1.5,
		TTSFormat:   "mp3",
	}
}

// Synthesize generates an audio file at outputPath using mmx speech synthesize.
// Text is passed via stdin (--text-file -) to avoid shell argument length limits.
// The caller is responsible for setting a deadline on ctx.
func (p *MiniMaxProvider) Synthesize(ctx context.Context, text string, outputPath string, language string) error {
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
	// Use per-request language if provided, otherwise fall back to configured default
	lang := language
	if lang == "" {
		lang = p.TTSLanguage
	}
	if lang != "" {
		args = append(args, "--language", lang)
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
		slog.String("language", lang),
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
