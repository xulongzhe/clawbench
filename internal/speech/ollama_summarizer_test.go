package speech

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewOllamaSummarizer_Defaults(t *testing.T) {
	s := NewOllamaSummarizer("", "")
	assert.Equal(t, "http://localhost:11434", s.BaseURL)
	assert.Equal(t, "gemma3:270m", s.Model)
	assert.NotNil(t, s.HTTPClient)
}

func TestNewOllamaSummarizer_CustomConfig(t *testing.T) {
	s := NewOllamaSummarizer("http://192.168.1.100:11434/", "llama3.2:1b")
	assert.Equal(t, "http://192.168.1.100:11434", s.BaseURL) // trailing slash trimmed
	assert.Equal(t, "llama3.2:1b", s.Model)
}

func TestOllamaSummarizer_ShortText(t *testing.T) {
	s := NewOllamaSummarizer("", "")
	// Text shorter than 300 chars should be returned as-is without any HTTP call
	result, err := s.Summarize(context.Background(), "这是一段短文本", "zh")
	assert.NoError(t, err)
	assert.Equal(t, "这是一段短文本", result)
}

func TestOllamaSummarizer_APICall(t *testing.T) {
	var receivedReq ollamaChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/chat", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		err := json.NewDecoder(r.Body).Decode(&receivedReq)
		assert.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ollamaChatResponse{
			Message: ollamaChatMessage{Role: "assistant", Content: "这是总结后的内容。"},
			Done:    true,
		})
	}))
	defer server.Close()

	s := NewOllamaSummarizer(server.URL, "gemma3:270m")
	longText := strings.Repeat("这是一段很长的AI回复内容，用于测试总结功能。", 20) // >300 chars
	result, err := s.Summarize(context.Background(), longText, "zh")

	assert.NoError(t, err)
	assert.Equal(t, "这是总结后的内容。", result)
	assert.Equal(t, "gemma3:270m", receivedReq.Model)
	assert.False(t, receivedReq.Stream)
	assert.Equal(t, "system", receivedReq.Messages[0].Role)
	assert.Equal(t, "user", receivedReq.Messages[1].Role)
	assert.Equal(t, 1024, receivedReq.Options.NumPredict)
}

func TestOllamaSummarizer_MultiPass(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var content string
		if callCount == 1 {
			// First pass: return a very long result (>4000 bytes) to trigger second pass
			content = strings.Repeat("这是一段很长的总结结果，", 300) // ~3600+ runes, >4000 bytes
		} else {
			// Second pass: return a short result
			content = "精简后的总结。"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ollamaChatResponse{
			Message: ollamaChatMessage{Role: "assistant", Content: content},
			Done:    true,
		})
	}))
	defer server.Close()

	s := NewOllamaSummarizer(server.URL, "gemma3:270m")
	longText := strings.Repeat("这是一段很长的AI回复内容，用于测试总结功能。", 20)
	result, err := s.Summarize(context.Background(), longText, "zh")

	assert.NoError(t, err)
	assert.Equal(t, "精简后的总结。", result)
	assert.Equal(t, 2, callCount)
}

func TestOllamaSummarizer_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "model not found")
	}))
	defer server.Close()

	s := NewOllamaSummarizer(server.URL, "nonexistent:model")
	longText := strings.Repeat("这是一段很长的AI回复内容，用于测试总结功能。", 20)
	_, err := s.Summarize(context.Background(), longText, "zh")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
	assert.Contains(t, err.Error(), "model not found")
}

func TestOllamaSummarizer_ConnectionRefused(t *testing.T) {
	// Use a port that's almost certainly not listening
	s := NewOllamaSummarizer("http://127.0.0.1:1", "gemma3:270m")
	s.HTTPClient.Timeout = 2 * time.Second

	longText := strings.Repeat("这是一段很长的AI回复内容，用于测试总结功能。", 20)
	_, err := s.Summarize(context.Background(), longText, "zh")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ollama request")
}

func TestOllamaSummarizer_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ollamaChatResponse{
			Message: ollamaChatMessage{Role: "assistant", Content: "  "},
			Done:    true,
		})
	}))
	defer server.Close()

	s := NewOllamaSummarizer(server.URL, "gemma3:270m")
	longText := strings.Repeat("这是一段很长的AI回复内容，用于测试总结功能。", 20)
	_, err := s.Summarize(context.Background(), longText, "zh")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty output")
}

func TestOllamaSummarizer_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(5 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ollamaChatResponse{
			Message: ollamaChatMessage{Role: "assistant", Content: "too late"},
			Done:    true,
		})
	}))
	defer server.Close()

	s := NewOllamaSummarizer(server.URL, "gemma3:270m")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	longText := strings.Repeat("这是一段很长的AI回复内容，用于测试总结功能。", 20)
	_, err := s.Summarize(ctx, longText, "zh")

	assert.Error(t, err)
}
