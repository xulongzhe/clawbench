package speech

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// OllamaSummarizer implements Summarizer using the Ollama HTTP API.
// It calls the native /api/chat endpoint with stream:false for simple
// request/response summarization. No external Go dependencies required.
type OllamaSummarizer struct {
	BaseURL    string       // Ollama API base URL (e.g., "http://localhost:11434")
	Model      string       // Model name (e.g., "gemma3:270m")
	HTTPClient *http.Client // Shared HTTP client with timeout
	gs         genericSummarizer
}

// NewOllamaSummarizer creates an OllamaSummarizer with the given configuration.
// Empty baseURL defaults to "http://localhost:11434".
// Empty model defaults to "gemma3:270m".
func NewOllamaSummarizer(baseURL, model string) *OllamaSummarizer {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "gemma3:270m"
	}
	s := &OllamaSummarizer{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Model:   model,
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second, // generous timeout for local inference
		},
	}
	s.gs = NewGenericSummarizer(s.doSummarizePass)
	return s
}

// ollamaChatRequest is the request body for POST /api/chat.
type ollamaChatRequest struct {
	Model    string              `json:"model"`
	Messages []ollamaChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
	Options  ollamaOptions       `json:"options,omitempty"`
}

type ollamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ollamaOptions controls model generation parameters.
type ollamaOptions struct {
	NumPredict  int     `json:"num_predict,omitempty"`  // Max tokens to generate
	Temperature float64 `json:"temperature,omitempty"` // Sampling temperature
}

// ollamaChatResponse is the response body for POST /api/chat (stream:false).
type ollamaChatResponse struct {
	Message ollamaChatMessage `json:"message"`
	Done    bool              `json:"done"`
}

// Summarize condenses text for voice output using the Ollama API.
func (s *OllamaSummarizer) Summarize(ctx context.Context, text string, language string) (string, error) {
	return s.gs.Summarize(ctx, text, language)
}

// doSummarizePass performs a single summarization pass using the Ollama /api/chat endpoint.
func (s *OllamaSummarizer) doSummarizePass(ctx context.Context, text, systemPrompt string, pass int) (string, error) {
	reqBody := ollamaChatRequest{
		Model: s.Model,
		Messages: []ollamaChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: text},
		},
		Stream: false,
		Options: ollamaOptions{
			NumPredict:  1024,
			Temperature: 0.3,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("ollama request marshal (pass %d): %w", pass, err)
	}

	url := s.BaseURL + "/api/chat"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("ollama request create (pass %d): %w", pass, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request (pass %d): %w", pass, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama API returned status %d (pass %d): %s", resp.StatusCode, pass, string(body))
	}

	var chatResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("ollama response decode (pass %d): %w", pass, err)
	}

	result := strings.TrimSpace(chatResp.Message.Content)
	if result == "" {
		return "", fmt.Errorf("ollama (pass %d) returned empty output", pass)
	}

	slog.Info("tts summarize pass completed",
		slog.Int("pass", pass),
		slog.String("backend", "ollama"),
		slog.String("model", s.Model),
		slog.Int("result_len", len([]rune(result))),
	)

	return result, nil
}
