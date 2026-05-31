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

// newMockEmbeddingServer creates a mock OpenAI-compatible HTTP server and an EmbeddingClient
// pointed at it. Returns (client, cleanup).
func newMockEmbeddingServer(t *testing.T, handler http.HandlerFunc) (*EmbeddingClient, func()) {
	t.Helper()
	server := httptest.NewServer(handler)
	client := NewEmbeddingClient(server.URL, "bge-m3", "")
	// Replace HTTPClient with one that has a short timeout for tests
	client.HTTPClient = server.Client()
	return client, server.Close
}

// ---------- NewEmbeddingClient ----------

func TestNewEmbeddingClient_TrimsSlash(t *testing.T) {
	c := NewEmbeddingClient("http://localhost:11434/", "bge-m3", "")
	assert.Equal(t, "http://localhost:11434", c.BaseURL, "trailing slash should be trimmed")
}

func TestNewEmbeddingClient_DefaultTimeout(t *testing.T) {
	c := NewEmbeddingClient("http://localhost:11434", "bge-m3", "")
	assert.Equal(t, 120, int(c.HTTPClient.Timeout.Seconds()), "default timeout should be 120s")
}

// ---------- Embed ----------

func TestEmbed_Success(t *testing.T) {
	client, cleanup := newMockEmbeddingServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/embeddings", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req openaiEmbedRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "bge-m3", req.Model)
		assert.Equal(t, []string{"hello world"}, req.Input)

		resp := openaiEmbedResponse{
			Data: []openaiEmbeddingData{
				{Embedding: makeTestEmbedding(1024), Index: 0},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	emb, err := client.Embed(context.Background(), "hello world")
	assert.NoError(t, err)
	assert.Len(t, emb, 1024)
}

func TestEmbed_Non200Status(t *testing.T) {
	client, cleanup := newMockEmbeddingServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	})
	defer cleanup()

	_, err := client.Embed(context.Background(), "hello")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestEmbed_EmptyEmbedding(t *testing.T) {
	client, cleanup := newMockEmbeddingServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := openaiEmbedResponse{
			Data: []openaiEmbeddingData{
				{Embedding: []float64{}, Index: 0},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	_, err := client.Embed(context.Background(), "hello")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty embedding")
}

func TestEmbed_InvalidJSON(t *testing.T) {
	client, cleanup := newMockEmbeddingServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	})
	defer cleanup()

	_, err := client.Embed(context.Background(), "hello")
	assert.Error(t, err)
}

// ---------- EmbedBatch ----------

func TestEmbedBatch_Success(t *testing.T) {
	client, cleanup := newMockEmbeddingServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := openaiEmbedResponse{
			Data: []openaiEmbeddingData{
				{Embedding: makeTestEmbedding(1024), Index: 0},
				{Embedding: makeTestEmbedding(1024), Index: 1},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	texts := []string{"hello", "world"}
	embeddings, err := client.EmbedBatch(context.Background(), texts)
	assert.NoError(t, err)
	assert.Len(t, embeddings, 2)
}

func TestEmbedBatch_ErrorResponse(t *testing.T) {
	client, cleanup := newMockEmbeddingServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	texts := []string{"hello", "world"}
	_, err := client.EmbedBatch(context.Background(), texts)
	assert.Error(t, err)
}

func TestEmbedBatch_Empty(t *testing.T) {
	client, cleanup := newMockEmbeddingServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not call server for empty batch")
	})
	defer cleanup()

	embeddings, err := client.EmbedBatch(context.Background(), []string{})
	assert.NoError(t, err)
	assert.Empty(t, embeddings)
}

// ---------- IsHealthy ----------

func TestIsHealthy_ReachableModelAvailable(t *testing.T) {
	client, cleanup := newMockEmbeddingServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/models", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		resp := openaiModelsResponse{
			Data: []openaiModelInfo{
				{ID: "bge-m3:latest"},
				{ID: "llama3:latest"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	reachable, modelAvailable, err := client.IsHealthy(context.Background())
	assert.NoError(t, err)
	assert.True(t, reachable, "server should be reachable")
	assert.True(t, modelAvailable, "model should be available")
}

func TestIsHealthy_ReachableModelWithPrefix(t *testing.T) {
	client, cleanup := newMockEmbeddingServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := openaiModelsResponse{
			Data: []openaiModelInfo{
				{ID: "bge-m3:latest"}, // matches with HasPrefix "bge-m3:"
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	reachable, modelAvailable, err := client.IsHealthy(context.Background())
	assert.NoError(t, err)
	assert.True(t, reachable)
	assert.True(t, modelAvailable, "model with :suffix should match")
}

func TestIsHealthy_ReachableModelNotAvailable(t *testing.T) {
	client, cleanup := newMockEmbeddingServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := openaiModelsResponse{
			Data: []openaiModelInfo{
				{ID: "llama3:latest"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	reachable, modelAvailable, err := client.IsHealthy(context.Background())
	assert.NoError(t, err)
	assert.True(t, reachable, "server should be reachable")
	assert.False(t, modelAvailable, "model should not be available")
}

func TestIsHealthy_NotReachable(t *testing.T) {
	// Point to a non-existent server
	client := NewEmbeddingClient("http://127.0.0.1:1", "bge-m3", "")

	reachable, modelAvailable, err := client.IsHealthy(context.Background())
	assert.NoError(t, err, "unreachable server should not return error")
	assert.False(t, reachable, "server should not be reachable")
	assert.False(t, modelAvailable, "model should not be available")
}

func TestIsHealthy_Non200Status(t *testing.T) {
	client, cleanup := newMockEmbeddingServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	defer cleanup()

	_, _, err := client.IsHealthy(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestIsHealthy_404AssumesHealthy(t *testing.T) {
	client, cleanup := newMockEmbeddingServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer cleanup()

	reachable, modelAvailable, err := client.IsHealthy(context.Background())
	assert.NoError(t, err)
	assert.True(t, reachable, "404 should be treated as reachable")
	assert.True(t, modelAvailable, "404 should assume model available")
}

func TestIsHealthy_InvalidJSON(t *testing.T) {
	client, cleanup := newMockEmbeddingServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	})
	defer cleanup()

	reachable, _, err := client.IsHealthy(context.Background())
	assert.Error(t, err)
	assert.True(t, reachable, "server was reachable (returned response)")
}

// ---------- Dim ----------

func TestEmbed_NoResultsFromBatch(t *testing.T) {
	client, cleanup := newMockEmbeddingServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Return empty data array
		resp := openaiEmbedResponse{Data: []openaiEmbeddingData{}}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	_, err := client.Embed(context.Background(), "hello")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty data")
}

func TestEmbedBatch_OutOfRangeIndex(t *testing.T) {
	client, cleanup := newMockEmbeddingServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := openaiEmbedResponse{
			Data: []openaiEmbeddingData{
				{Embedding: makeTestEmbedding(4), Index: 99}, // out of range
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	_, err := client.EmbedBatch(context.Background(), []string{"hello"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out-of-range index")
}

func TestEmbedBatch_WithAPIKey(t *testing.T) {
	client, cleanup := newMockEmbeddingServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		resp := openaiEmbedResponse{
			Data: []openaiEmbeddingData{
				{Embedding: makeTestEmbedding(4), Index: 0},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()
	client.APIKey = "test-key"

	_, err := client.EmbedBatch(context.Background(), []string{"hello"})
	assert.NoError(t, err)
}

func TestIsHealthy_WithAPIKey(t *testing.T) {
	client, cleanup := newMockEmbeddingServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		resp := openaiModelsResponse{Data: []openaiModelInfo{{ID: "bge-m3:latest"}}}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()
	client.APIKey = "test-key"

	reachable, modelAvailable, err := client.IsHealthy(context.Background())
	assert.NoError(t, err)
	assert.True(t, reachable)
	assert.True(t, modelAvailable)
}

func TestIsHealthy_RequestCreationError(t *testing.T) {
	// Use an invalid URL that causes NewRequestWithContext to fail
	client := NewEmbeddingClient("http://localhost:11434", "bge-m3", "")
	// Cancel context immediately to force request error
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	reachable, modelAvailable, err := client.IsHealthy(ctx)
	// Cancelled context causes Do() error, which returns (false, false, nil)
	assert.NoError(t, err)
	assert.False(t, reachable)
	assert.False(t, modelAvailable)
}

func TestDim_Initial(t *testing.T) {
	client := NewEmbeddingClient("http://localhost:11434", "bge-m3", "")
	assert.Equal(t, 0, client.Dim(), "dim should be 0 before any embed call")
}

func TestEmbedBatch_EmptyEmbeddingAtIndex(t *testing.T) {
	client, cleanup := newMockEmbeddingServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := openaiEmbedResponse{
			Data: []openaiEmbeddingData{
				{Embedding: []float64{}, Index: 0}, // empty embedding at valid index
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	_, err := client.EmbedBatch(context.Background(), []string{"hello"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty embedding at index")
}

func TestEmbedBatch_EmptyData(t *testing.T) {
	client, cleanup := newMockEmbeddingServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := openaiEmbedResponse{Data: []openaiEmbeddingData{}}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer cleanup()

	_, err := client.EmbedBatch(context.Background(), []string{"hello"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty data")
}
