package summarize

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- NewMMX ---

func TestNewMMX_DefaultModel(t *testing.T) {
	s := NewMMX()
	assert.Equal(t, "MiniMax-M2.7", s.Model)
}

func TestNewMMX_CustomModel(t *testing.T) {
	s := NewMMX()
	s.Model = "custom-model"
	assert.Equal(t, "custom-model", s.Model)
}

// --- MMXSummarizer.Summarize short text (no CLI needed) ---

func TestMMXSummarize_ShortText_NoCLI(t *testing.T) {
	s := NewMMX()
	result, err := s.Summarize(context.Background(), "短文本无需总结", "zh")
	assert.NoError(t, err)
	assert.Equal(t, "短文本无需总结", result)
}

func TestMMXSummarize_ShortTextWithMarkdown_NoCLI(t *testing.T) {
	s := NewMMX()
	result, err := s.Summarize(context.Background(), "Hello **world** and *test*.", "en")
	assert.NoError(t, err)
	assert.NotContains(t, result, "**")
	assert.NotContains(t, result, "*")
}

// --- MMXSummarizer.DoSummarizePass ---

func TestMMXSummarize_doSummarizePass_CLIUnavailable(t *testing.T) {
	if _, err := exec.LookPath("mmx"); err == nil {
		t.Skip("mmx CLI available, skipping CLI-unavailable test")
	}

	s := NewMMX()
	_, err := s.DoSummarizePass(context.Background(), "some text", "system prompt", 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mmx text chat")
}

func TestMMXSummarize_doSummarizePass_CancelledContext(t *testing.T) {
	s := NewMMX()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := s.DoSummarizePass(ctx, "some text", "system prompt", 1)
	assert.Error(t, err)
}

// --- MMXSummarizer long text integration (requires mmx CLI) ---

func TestMMXSummarize_LongText_WithCLI(t *testing.T) {
	if _, err := exec.LookPath("mmx"); err != nil {
		t.Skip("mmx CLI not available, skipping integration test")
	}

	s := NewMMX()
	longText := strings.Repeat("这是一段较长的AI回复内容，包含了详细的技术分析和代码示例。", 10)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := s.Summarize(ctx, longText, "zh")
	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Less(t, len([]rune(result)), len([]rune(longText)))
}

// --- MMXSummarizer.DoSummarizePass with custom model ---

func TestMMXSummarize_doSummarizePass_CustomModel_WithCLI(t *testing.T) {
	if _, err := exec.LookPath("mmx"); err != nil {
		t.Skip("mmx CLI not available, skipping integration test")
	}

	s := NewMMX()
	s.Model = "MiniMax-M2.7"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := s.DoSummarizePass(ctx, "请简要总结这段文字。", "你是一个总结助手。", 1)
	if err != nil {
		// May fail for various reasons (rate limit, model unavailable), just log
		t.Logf("doSummarizePass returned error (may be expected): %v", err)
		return
	}
	assert.NotEmpty(t, result)
}

// --- MMXSummarizer pipeline test with mock pass function ---

func TestMMXSummarize_Pipeline_ReSummarization(t *testing.T) {
	callCount := 0
	// Use ttsPipeline with mock pass function
	s := ttsPipeline{
		passFn: func(ctx context.Context, text, systemPrompt string, pass int) (string, error) {
			callCount++
			if pass == 1 {
				// Return a long result to trigger re-summarization
				return strings.Repeat("长总结结果。", 500), nil
			}
			return "精简结果", nil
		},
		basePrompt: "Base prompt",
	}

	longText := strings.Repeat("这是一段很长的AI回复内容。", 30)
	result, err := s.Summarize(context.Background(), longText, "zh")
	assert.NoError(t, err)
	assert.Equal(t, "精简结果", result)
	assert.Equal(t, 2, callCount)
}

func TestMMXSummarize_Pipeline_PassError(t *testing.T) {
	s := ttsPipeline{
		passFn: func(ctx context.Context, text, systemPrompt string, pass int) (string, error) {
			return "", fmt.Errorf("summarization service unavailable")
		},
		basePrompt: "Base prompt",
	}

	longText := strings.Repeat("这是一段很长的AI回复内容。", 30)
	_, err := s.Summarize(context.Background(), longText, "zh")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "summarization service unavailable")
}
