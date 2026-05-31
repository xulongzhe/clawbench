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

// --- DRM token generation tests ---

func TestGenerateSecMsGec_Format(t *testing.T) {
	token := generateSecMsGec()
	// Should be a 64-character uppercase hex string (SHA256)
	assert.Len(t, token, 64)
	for _, c := range token {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F'),
			"token should be uppercase hex, got: %c", c)
	}
}

func TestGenerateSecMsGec_ConsistentWithin5Min(t *testing.T) {
	token1 := generateSecMsGec()
	token2 := generateSecMsGec()
	// Tokens should be identical within the same 5-minute window
	assert.Equal(t, token1, token2, "tokens within 5-min window should be identical")
}

func TestGenerateMUID_Format(t *testing.T) {
	muid := generateMUID()
	// Should be a 32-character uppercase hex string
	assert.Len(t, muid, 32)
	for _, c := range muid {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F'),
			"muid should be uppercase hex, got: %c", c)
	}
}

func TestGenerateMUID_Uniqueness(t *testing.T) {
	muid1 := generateMUID()
	muid2 := generateMUID()
	assert.NotEqual(t, muid1, muid2, "different MUIDs should be unique")
}

func TestGenerateConnectID_Format(t *testing.T) {
	id := generateConnectID()
	// UUID v4 without dashes = 32 hex chars
	assert.Len(t, id, 32)
	for _, c := range id {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"connect ID should be lowercase hex, got: %c", c)
	}
}

func TestGenerateConnectID_Uniqueness(t *testing.T) {
	id1 := generateConnectID()
	id2 := generateConnectID()
	assert.NotEqual(t, id1, id2, "different connect IDs should be unique")
}

// --- SSML building tests ---

func TestBuildSSML(t *testing.T) {
	tests := []struct {
		name     string
		voice    string
		rate     string
		text     string
		contains []string
	}{
		{
			"Chinese voice",
			"zh-CN-XiaoxiaoNeural", "+0%", "你好世界",
			[]string{"zh-CN-XiaoxiaoNeural", "+0%", "你好世界", "<speak", "</speak>", "<voice", "</voice>", "<prosody", "</prosody>"},
		},
		{
			"English voice with fast rate",
			"en-US-JennyNeural", "+20%", "Hello world",
			[]string{"en-US-JennyNeural", "+20%", "Hello world"},
		},
		{
			"XML special characters escaped",
			"zh-CN-XiaoxiaoNeural", "+0%", "A<B&C>D",
			[]string{"A&lt;B&amp;C&gt;D"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ssml := buildSSML(tt.voice, tt.rate, tt.text)
			for _, substr := range tt.contains {
				assert.Contains(t, ssml, substr, "SSML should contain %q", substr)
			}
		})
	}
}

// --- removeIncompatibleChars tests ---

func TestRemoveIncompatibleChars(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"normal text", "hello world", "hello world"},
		{"tab preserved", "hello\tworld", "hello\tworld"},
		{"newline preserved", "hello\nworld", "hello\nworld"},
		{"carriage return preserved", "hello\rworld", "hello\rworld"},
		{"control chars removed", "hello\x00\x01world", "hello  world"},
		{"vertical tab removed", "hello\x0Bworld", "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeIncompatibleChars(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}
