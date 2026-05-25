package summarize

import (
	"context"
	"strings"
	"testing"

	"clawbench/internal/ai"
	"clawbench/internal/model"

	"github.com/stretchr/testify/assert"
)

// --- TaskSummarizer short text ---

func TestTaskSummarizer_ShortText(t *testing.T) {
	// Short text should return empty string (no summarization needed)
	s := &TaskSummarizer{
		Backend: &mockTaskBackend{},
	}

	result, err := s.Summarize(context.Background(), "短文本", "")
	assert.NoError(t, err)
	assert.Equal(t, "", result) // empty = no summarization needed
}

// --- TaskSummarizer long text via backend ---

type mockTaskBackend struct {
	streamCh     chan ai.StreamEvent
	executeErr   error
	executeCalled bool
	capturedReq  ai.ChatRequest
}

func (m *mockTaskBackend) Name() string { return "mock-task-backend" }

func (m *mockTaskBackend) ExecuteStream(ctx context.Context, req ai.ChatRequest) (<-chan ai.StreamEvent, error) {
	m.executeCalled = true
	m.capturedReq = req
	if m.executeErr != nil {
		return nil, m.executeErr
	}
	return m.streamCh, nil
}

func TestTaskSummarizer_LongText_ViaBackend(t *testing.T) {
	ch := make(chan ai.StreamEvent, 3)
	ch <- ai.StreamEvent{Type: "content", Content: "## 总结\n\n这是**精简**总结。"}
	ch <- ai.StreamEvent{Type: "content", Content: "\n\n```go\nfmt.Println()```"}
	ch <- ai.StreamEvent{Type: "done"}
	close(ch)

	mock := &mockTaskBackend{streamCh: ch}
	s := &TaskSummarizer{
		Backend: mock,
		model:   "test-model",
	}

	longText := strings.Repeat("这是一段较长的AI回复内容，包含了详细的技术分析。", 30)
	result, err := s.Summarize(context.Background(), longText, "")

	assert.NoError(t, err)
	assert.Contains(t, result, "总结")
	assert.Contains(t, result, "**精简**") // Markdown preserved
	assert.Contains(t, result, "```go")     // Code block preserved
	assert.True(t, mock.executeCalled)
	assert.Equal(t, "test-model", mock.capturedReq.Model)
	assert.Equal(t, taskSummarizePrompt, mock.capturedReq.SystemPrompt)
}

func TestTaskSummarizer_BackendError(t *testing.T) {
	mock := &mockTaskBackend{
		executeErr: context.DeadlineExceeded,
	}
	s := &TaskSummarizer{
		Backend: mock,
	}

	longText := strings.Repeat("这是一段较长的AI回复内容。", 30)
	_, err := s.Summarize(context.Background(), longText, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "task summarization backend")
}

func TestTaskSummarizer_StreamError(t *testing.T) {
	ch := make(chan ai.StreamEvent, 1)
	ch <- ai.StreamEvent{Type: "error", Error: "out of tokens"}
	close(ch)

	mock := &mockTaskBackend{streamCh: ch}
	s := &TaskSummarizer{
		Backend: mock,
	}

	longText := strings.Repeat("这是一段较长的AI回复内容。", 30)
	_, err := s.Summarize(context.Background(), longText, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of tokens")
}

func TestTaskSummarizer_EmptyOutput(t *testing.T) {
	ch := make(chan ai.StreamEvent, 1)
	ch <- ai.StreamEvent{Type: "done"}
	close(ch)

	mock := &mockTaskBackend{streamCh: ch}
	s := &TaskSummarizer{
		Backend: mock,
	}

	longText := strings.Repeat("这是一段较长的AI回复内容。", 30)
	_, err := s.Summarize(context.Background(), longText, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty output")
}

func TestTaskSummarizer_Truncation(t *testing.T) {
	origMax := MaxSummarizeRunes
	MaxSummarizeRunes = 100
	defer func() { MaxSummarizeRunes = origMax }()

	ch := make(chan ai.StreamEvent, 2)
	ch <- ai.StreamEvent{Type: "content", Content: "总结结果"}
	ch <- ai.StreamEvent{Type: "done"}
	close(ch)

	mock := &mockTaskBackend{streamCh: ch}
	s := &TaskSummarizer{
		Backend: mock,
	}

	longText := strings.Repeat("长文本", 200) // 600 runes
	result, err := s.Summarize(context.Background(), longText, "")
	assert.NoError(t, err)
	assert.Equal(t, "总结结果", result)
	// Verify truncation happened
	assert.LessOrEqual(t, len([]rune(mock.capturedReq.Prompt)), 100)
}

// --- TaskSummarizer via pipeline (API backend) ---

func TestTaskSummarizer_ViaPipeline(t *testing.T) {
	// Create a pipeline with PreserveMarkdown=true and task prompt
	var capturedPrompt string
	passFn := func(ctx context.Context, text, systemPrompt string, pass int) (string, error) {
		capturedPrompt = systemPrompt
		return "## 保留格式的总结", nil
	}

	pipeline := NewPipelineWithOpts(passFn, taskSummarizePrompt, SummarizeOption{PreserveMarkdown: true})
	s := NewTaskSummarizerFromPipeline(pipeline)

	longText := strings.Repeat("这是一段较长的AI回复内容。", 30)
	result, err := s.Summarize(context.Background(), longText, "")

	assert.NoError(t, err)
	assert.Contains(t, result, "保留格式")
	assert.Contains(t, capturedPrompt, "精简总结")
}

func TestTaskSummarizer_ViaPipeline_ShortText(t *testing.T) {
	passFn := func(ctx context.Context, text, systemPrompt string, pass int) (string, error) {
		return "should not be called", nil
	}

	pipeline := NewPipelineWithOpts(passFn, taskSummarizePrompt, SummarizeOption{PreserveMarkdown: true})
	s := NewTaskSummarizerFromPipeline(pipeline)

	result, err := s.Summarize(context.Background(), "短文本", "")
	assert.NoError(t, err)
	// TaskSummarizer returns empty string for short text (meaning "no summarization needed")
	assert.Equal(t, "", result)
}

// --- NewTaskSummarizer constructor ---

func TestNewTaskSummarizer_UnsupportedBackend(t *testing.T) {
	_, err := NewTaskSummarizer("nonexistent_backend_type", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create AI backend")
}

// --- ExtractTextFromBlocks ---

func TestExtractTextFromBlocks_TextOnly(t *testing.T) {
	blocks := []model.ContentBlock{
		{Type: "text", Text: "Hello world"},
		{Type: "text", Text: "Second paragraph"},
	}
	result := ExtractTextFromBlocks(blocks)
	assert.Equal(t, "Hello world\n\nSecond paragraph", result)
}

func TestExtractTextFromBlocks_SkipsNonText(t *testing.T) {
	blocks := []model.ContentBlock{
		{Type: "text", Text: "Important content"},
		{Type: "thinking", Text: "internal reasoning"},
		{Type: "tool_use", Name: "Bash", ID: "1"},
		{Type: "warning", Text: "some warning"},
		{Type: "error", Text: "some error"},
		{Type: "text", Text: "More content"},
	}
	result := ExtractTextFromBlocks(blocks)
	assert.Equal(t, "Important content\n\nMore content", result)
}

func TestExtractTextFromBlocks_Empty(t *testing.T) {
	blocks := []model.ContentBlock{}
	result := ExtractTextFromBlocks(blocks)
	assert.Equal(t, "", result)
}

func TestExtractTextFromBlocks_NoTextBlocks(t *testing.T) {
	blocks := []model.ContentBlock{
		{Type: "tool_use", Name: "Read", ID: "1"},
		{Type: "thinking", Text: "hmm"},
	}
	result := ExtractTextFromBlocks(blocks)
	assert.Equal(t, "", result)
}

func TestExtractTextFromBlocks_EmptyTextSkipped(t *testing.T) {
	blocks := []model.ContentBlock{
		{Type: "text", Text: "Content"},
		{Type: "text", Text: ""}, // empty text should be skipped
		{Type: "text", Text: "More"},
	}
	result := ExtractTextFromBlocks(blocks)
	assert.Equal(t, "Content\n\nMore", result)
}
