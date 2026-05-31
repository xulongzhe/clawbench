package speech

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEdgeTTSProvider_Defaults(t *testing.T) {
	p := NewEdgeTTSProvider()
	assert.Equal(t, edgeDefaultVoice, p.Voice)
	assert.Equal(t, "+0%", p.Rate)
}

func TestEdgeTTSProvider_Synthesize_CancelledContext(t *testing.T) {
	p := NewEdgeTTSProvider()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	outputPath := filepath.Join(t.TempDir(), "output.mp3")
	err := p.Synthesize(ctx, "hello", outputPath, "zh")
	assert.Error(t, err)
}

func TestEdgeTTSProvider_Synthesize_CreatesDirectory(t *testing.T) {
	p := NewEdgeTTSProvider()

	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "nested", "deep")
	outputPath := filepath.Join(nestedDir, "output.mp3")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := p.Synthesize(ctx, "hello", outputPath, "zh")

	require.Error(t, err)
	_, statErr := os.Stat(nestedDir)
	assert.NoError(t, statErr, "output directory should be created even if synthesis fails")
}

// --- EdgeTTSProvider rate handling ---

func TestEdgeTTSProvider_RateSetting(t *testing.T) {
	tests := []struct {
		name        string
		rate        string
		expectEmpty bool // whether rate should effectively be a no-op
	}{
		{"default rate +0%", "+0%", true},
		{"empty rate", "", true},
		{"faster rate +20%", "+20%", false},
		{"slower rate -10%", "-10%", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &EdgeTTSProvider{
				Voice: "zh-CN-XiaoxiaoNeural",
				Rate:  tt.rate,
			}
			assert.Equal(t, tt.rate, p.Rate)
			isNoOp := p.Rate == "" || p.Rate == "+0%"
			assert.Equal(t, tt.expectEmpty, isNoOp)
		})
	}
}

// --- EdgeTTSProvider different voices ---

func TestEdgeTTSProvider_DifferentVoices(t *testing.T) {
	voices := []string{
		"zh-CN-XiaoxiaoNeural",
		"en-US-JennyNeural",
		"ja-JP-NanamiNeural",
		"ko-KR-SunHiNeural",
	}

	for _, voice := range voices {
		p := &EdgeTTSProvider{Voice: voice, Rate: "+0%"}
		assert.Equal(t, voice, p.Voice)
	}
}
