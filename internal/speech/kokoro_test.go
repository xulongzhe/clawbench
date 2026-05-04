package speech

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// --- Shared Summarizer: short text bypass (tests genericSummarizer) ---

func TestGenericSummarizer_ShortText_BypassesLLM(t *testing.T) {
	s := NewMMXSummarizer()
	shortText := "这是一个简短的消息，不需要总结。"
	result, err := s.Summarize(context.Background(), shortText, "zh")
	assert.NoError(t, err)
	assert.Contains(t, result, "简短的消息")
}

func TestGenericSummarizer_ShortTextWithMarkdown_StripsMarkdown(t *testing.T) {
	s := NewMMXSummarizer()
	input := "Short **bold** and *italic* text."
	result, err := s.Summarize(context.Background(), input, "zh")
	assert.NoError(t, err)
	assert.NotContains(t, result, "**")
	assert.NotContains(t, result, "*")
	assert.Contains(t, result, "bold")
	assert.Contains(t, result, "italic")
}

// --- Shared Summarizer: long text (requires mmx CLI, skip if unavailable) ---

func TestGenericSummarizer_LongText_WithCLI(t *testing.T) {
	if _, err := exec.LookPath("mmx"); err != nil {
		t.Skip("mmx CLI not available, skipping integration test")
	}

	s := NewMMXSummarizer()
	longText := strings.Repeat("这是一个较长的AI回复内容，包含了详细的技术分析和代码示例。", 10)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	result, err := s.Summarize(ctx, longText, "zh")
	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Less(t, len([]rune(result)), len([]rune(longText)))
}

// --- Shared Summarizer: context cancellation ---

func TestGenericSummarizer_CancelledContext(t *testing.T) {
	s := NewMMXSummarizer()
	longText := strings.Repeat("这是需要被总结的长文本内容。", 50)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := s.Summarize(ctx, longText, "zh")
	assert.Error(t, err)
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
