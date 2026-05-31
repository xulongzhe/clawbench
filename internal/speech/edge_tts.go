package speech

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	edgetts "github.com/lib-x/edgetts"
)

const (
	// edgeDefaultVoice is the default Chinese voice for edge-tts.
	edgeDefaultVoice = "zh-CN-XiaoxiaoNeural"
)

// EdgeTTSProvider implements SpeechProvider using edge-tts (Microsoft Edge TTS).
// Uses a native Go library — no external CLI or Python dependency required.
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

// Synthesize generates audio from text using Microsoft Edge TTS and writes to outputPath.
func (p *EdgeTTSProvider) Synthesize(ctx context.Context, text string, outputPath string, language string) error {
	// Ensure output directory exists
	if err := os.MkdirAll(outputPath[:len(outputPath)-len(lastPart(outputPath))], 0755); err != nil {
		return fmt.Errorf("edge-tts: failed to create output directory: %w", err)
	}

	opts := []edgetts.Option{
		edgetts.WithVoice(p.Voice),
	}
	if p.Rate != "" && p.Rate != "+0%" {
		opts = append(opts, edgetts.WithRate(p.Rate))
	}

	client := edgetts.New(opts...)

	if err := client.Save(ctx, text, outputPath); err != nil {
		return fmt.Errorf("edge-tts: %w", err)
	}

	slog.Info("edge-tts synthesize completed",
		slog.String("output", outputPath),
		slog.Int("text_len", len([]rune(text))),
	)
	return nil
}

// lastPart returns the last path component (filename).
func lastPart(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}
