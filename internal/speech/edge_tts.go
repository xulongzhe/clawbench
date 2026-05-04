package speech

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	// edgeTTSCmd is the path to the edge-tts executable.
	edgeTTSCmd = ".venv/bin/edge-tts"

	// edgeDefaultVoice is the default Chinese voice for edge-tts.
	edgeDefaultVoice = "zh-CN-XiaoxiaoNeural"
)

// EdgeTTSProvider implements SpeechProvider using edge-tts (Microsoft Edge TTS).
// edge-tts is free, has no quota limits, and provides high-quality Chinese voices.
type EdgeTTSProvider struct {
	// Voice is the edge-tts voice ID (default: "zh-CN-XiaoxiaoNeural").
	Voice string
	// Rate is the speech speed adjustment (e.g. "+0%", "+20%", "-10%").
	Rate string
}

// NewEdgeTTSProvider creates an EdgeTTSProvider with sensible defaults.
func NewEdgeTTSProvider() *EdgeTTSProvider {
	return &EdgeTTSProvider{
		Voice: edgeDefaultVoice,
		Rate:  "+0%",
	}
}

// Synthesize generates an audio file at outputPath using edge-tts.
// Text is written to a temp file and passed via --file to avoid shell argument limits.
func (p *EdgeTTSProvider) Synthesize(ctx context.Context, text string, outputPath string, _ string) error {
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", dir, err)
	}

	// Write text to a temp file to avoid shell argument length limits
	tmpFile, err := os.CreateTemp("", "edge-tts-input-*.txt")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(text); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	// Resolve edge-tts path relative to binary directory
	edgePath := edgeTTSCmd
	if exePath, err := os.Executable(); err == nil {
		edgePath = filepath.Join(filepath.Dir(exePath), edgeTTSCmd)
	}

	args := []string{
		"--voice", p.Voice,
		"--file", tmpPath,
		"--write-media", outputPath,
	}

	if p.Rate != "" && p.Rate != "+0%" {
		args = append(args, "--rate", p.Rate)
	}

	slog.Info("edge-tts synthesize",
		slog.String("output", outputPath),
		slog.String("voice", p.Voice),
		slog.String("rate", p.Rate),
		slog.Int("text_len", len(text)),
	)

	cmd := exec.CommandContext(ctx, edgePath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("edge-tts failed: %w (stderr: %s)", err, stderr.String())
	}

	if _, err := os.Stat(outputPath); err != nil {
		return fmt.Errorf("output file not created: %s", outputPath)
	}

	return nil
}
