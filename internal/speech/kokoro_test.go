package speech

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- NewKokoroProvider defaults ---

func TestNewKokoroProvider_Defaults(t *testing.T) {
	p := NewKokoroProvider()
	assert.Equal(t, "", p.ModelPath)  // Must be configured or resolved
	assert.Equal(t, "", p.VoicesPath) // Must be configured or resolved
	assert.Equal(t, "zf_001", p.Voice)
	assert.Equal(t, "cmn", p.Lang)
	assert.Equal(t, 1.0, p.Speed)
}

// --- ResolveKokoroPaths ---

func TestResolveKokoroPaths_Defaults(t *testing.T) {
	model, voices := ResolveKokoroPaths("", "")
	assert.Equal(t, filepath.Join(".clawbench/kokoro-models", "kokoro-v1.1-zh.onnx"), model)
	assert.Equal(t, filepath.Join(".clawbench/kokoro-models", "voices-v1.1-zh.bin"), voices)
}

func TestResolveKokoroPaths_Explicit(t *testing.T) {
	model, voices := ResolveKokoroPaths("/custom/model.onnx", "/custom/voices.bin")
	assert.Equal(t, "/custom/model.onnx", model)
	assert.Equal(t, "/custom/voices.bin", voices)
}

// --- Synthesize missing model ---

func TestKokoroSynthesize_MissingModel(t *testing.T) {
	p := NewKokoroProvider()
	p.ModelPath = "/nonexistent/kokoro-v1.0.onnx"
	p.VoicesPath = "/nonexistent/voices-v1.0.bin"
	outputPath := filepath.Join(t.TempDir(), "output.wav")

	err := p.Synthesize(context.Background(), "test", outputPath, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "model file not found")
}

// --- Synthesize no model configured ---

func TestKokoroSynthesize_NoModelConfigured(t *testing.T) {
	p := NewKokoroProvider()
	outputPath := filepath.Join(t.TempDir(), "output.wav")

	err := p.Synthesize(context.Background(), "test", outputPath, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "model path not configured")
}

// --- Synthesize context cancellation ---

func TestKokoroSynthesize_CancelledContext(t *testing.T) {
	p := NewKokoroProvider()
	p.ModelPath = "/fake/model.onnx"
	p.VoicesPath = "/fake/voices.bin"
	outputPath := filepath.Join(t.TempDir(), "cancelled.wav")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := p.Synthesize(ctx, "test", outputPath, "")
	assert.Error(t, err)
}
