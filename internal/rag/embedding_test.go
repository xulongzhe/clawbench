package rag

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- Helpers ----------

// newMockOllamaServer creates a mock Ollama HTTP server and an EmbeddingClient
// pointed at it. Returns (client, cleanup).
func newMockOllamaServer(t *testing.T, handler http.HandlerFunc) (*EmbeddingClient, func()) {
	t.Helper()
	server := httptest.NewServer(handler)
	client := NewEmbeddingClient(server.URL, "bge-m3")
	// Replace HTTPClient with one that has a short timeout for tests
	client.HTTPClient = server.Client()
	return client, server.Close
}

// ---------- NewEmbeddingClient ----------

func TestNewEmbeddingClient_TrimsSlash(t *testing.T) {
	c := NewEmbeddingClient("http://localhost:11434/", "bge-m3")
	assert.Equal(t, "http://localhost:11434", c.BaseURL, "trailing slash should be trimmed")
}

func TestNewEmbeddingClient_DefaultTimeout(t *testing.T) {
	c := NewEmbeddingClient("http://localhost:11434", "bge-m3")
	assert.Equal(t, 120, int(c.HTTPClient.Timeout.Seconds()), "default timeout should be 120s")
}

// ---------- Embed ----------

func TestEmbed_Success(t *testing.T) {
	client, cleanup := newMockOllamaServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/embeddings", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req ollamaEmbedRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "bge-m3", req.Model)
		assert.Equal(t, "hello world", req.Prompt)

		resp := ollamaEmbedResponse{
			Embedding: makeTestEmbedding(1024),
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	emb, err := client.Embed(context.Background(), "hello world")
	assert.NoError(t, err)
	assert.Len(t, emb, 1024)
}

func TestEmbed_Non200Status(t *testing.T) {
	client, cleanup := newMockOllamaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	})
	defer cleanup()

	_, err := client.Embed(context.Background(), "hello")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestEmbed_EmptyEmbedding(t *testing.T) {
	client, cleanup := newMockOllamaServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaEmbedResponse{Embedding: []float64{}}
		json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	_, err := client.Embed(context.Background(), "hello")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty embedding")
}

func TestEmbed_InvalidJSON(t *testing.T) {
	client, cleanup := newMockOllamaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	})
	defer cleanup()

	_, err := client.Embed(context.Background(), "hello")
	assert.Error(t, err)
}

// ---------- EmbedBatch ----------

func TestEmbedBatch_Success(t *testing.T) {
	callCount := 0
	client, cleanup := newMockOllamaServer(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		resp := ollamaEmbedResponse{Embedding: makeTestEmbedding(1024)}
		json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	texts := []string{"hello", "world"}
	embeddings, err := client.EmbedBatch(context.Background(), texts)
	assert.NoError(t, err)
	assert.Len(t, embeddings, 2)
	assert.Equal(t, 2, callCount, "should call Embed once per text")
}

func TestEmbedBatch_ErrorOnFirst(t *testing.T) {
	client, cleanup := newMockOllamaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	texts := []string{"hello", "world"}
	_, err := client.EmbedBatch(context.Background(), texts)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "embed text 1/2")
}

func TestEmbedBatch_ErrorOnSecond(t *testing.T) {
	callCount := 0
	client, cleanup := newMockOllamaServer(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			resp := ollamaEmbedResponse{Embedding: makeTestEmbedding(1024)}
			json.NewEncoder(w).Encode(resp)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	defer cleanup()

	texts := []string{"hello", "world"}
	_, err := client.EmbedBatch(context.Background(), texts)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "embed text 2/2")
}

func TestEmbedBatch_Empty(t *testing.T) {
	client, cleanup := newMockOllamaServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not call server for empty batch")
	})
	defer cleanup()

	embeddings, err := client.EmbedBatch(context.Background(), []string{})
	assert.NoError(t, err)
	assert.Empty(t, embeddings)
}

// ---------- IsHealthy ----------

func TestIsHealthy_ReachableModelAvailable(t *testing.T) {
	client, cleanup := newMockOllamaServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/tags", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		resp := ollamaTagsResponse{
			Models: []ollamaModelInfo{
				{Name: "bge-m3:latest"},
				{Name: "llama3:latest"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	reachable, modelAvailable, err := client.IsHealthy(context.Background())
	assert.NoError(t, err)
	assert.True(t, reachable, "server should be reachable")
	assert.True(t, modelAvailable, "model should be available")
}

func TestIsHealthy_ReachableModelWithPrefix(t *testing.T) {
	client, cleanup := newMockOllamaServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaTagsResponse{
			Models: []ollamaModelInfo{
				{Name: "bge-m3:latest"}, // matches with HasPrefix "bge-m3:"
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	reachable, modelAvailable, err := client.IsHealthy(context.Background())
	assert.NoError(t, err)
	assert.True(t, reachable)
	assert.True(t, modelAvailable, "model with :suffix should match")
}

func TestIsHealthy_ReachableModelNotAvailable(t *testing.T) {
	client, cleanup := newMockOllamaServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaTagsResponse{
			Models: []ollamaModelInfo{
				{Name: "llama3:latest"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	reachable, modelAvailable, err := client.IsHealthy(context.Background())
	assert.NoError(t, err)
	assert.True(t, reachable, "server should be reachable")
	assert.False(t, modelAvailable, "model should not be available")
}

func TestIsHealthy_NotReachable(t *testing.T) {
	// Point to a non-existent server
	client := NewEmbeddingClient("http://127.0.0.1:1", "bge-m3")

	reachable, modelAvailable, err := client.IsHealthy(context.Background())
	assert.NoError(t, err, "unreachable server should not return error")
	assert.False(t, reachable, "server should not be reachable")
	assert.False(t, modelAvailable, "model should not be available")
}

func TestIsHealthy_Non200Status(t *testing.T) {
	client, cleanup := newMockOllamaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	defer cleanup()

	_, _, err := client.IsHealthy(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestIsHealthy_InvalidJSON(t *testing.T) {
	client, cleanup := newMockOllamaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	})
	defer cleanup()

	reachable, _, err := client.IsHealthy(context.Background())
	assert.Error(t, err)
	assert.True(t, reachable, "server was reachable (returned response)")
}
