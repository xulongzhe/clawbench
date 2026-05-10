package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"clawbench/internal/rag"
	"clawbench/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- ServeRAGSearch ----------

func TestServeRAGSearch_MethodNotAllowed(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/rag/search", nil)
	w := callHandlerWithAuth(ServeRAGSearch, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestServeRAGSearch_EmptyQuery(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodPost, "/api/rag/search", map[string]any{"q": ""})
	w := callHandlerWithAuth(ServeRAGSearch, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServeRAGSearch_MissingQuery(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodPost, "/api/rag/search", map[string]any{})
	w := callHandlerWithAuth(ServeRAGSearch, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServeRAGSearch_NilStoreReturns503(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	// With nil GlobalStore/GlobalEmbedder, RAGSearch should return 503
	origStore := rag.GlobalStore
	origEmbedder := rag.GlobalEmbedder
	t.Cleanup(func() {
		rag.GlobalStore = origStore
		rag.GlobalEmbedder = origEmbedder
	})
	rag.GlobalStore = nil
	rag.GlobalEmbedder = nil

	req := newRequest(t, http.MethodPost, "/api/rag/search", map[string]any{"q": "test"})
	w := callHandlerWithAuth(ServeRAGSearch, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestServeRAGSearch_EmptyResultsArray(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	// Setup a real DuckDB store + mock embedder
	origStore := rag.GlobalStore
	origEmbedder := rag.GlobalEmbedder
	t.Cleanup(func() {
		rag.GlobalStore = origStore
		rag.GlobalEmbedder = origEmbedder
	})

	store := setupRAGStore(t)
	rag.GlobalStore = store
	// Use a mock server that returns valid embeddings
	embedder := setupWorkingMockEmbedder(t)
	rag.GlobalEmbedder = embedder

	req := newRequest(t, http.MethodPost, "/api/rag/search", map[string]any{"q": "test"})
	w := callHandlerWithAuth(ServeRAGSearch, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	// Results should be an empty array, not null
	results, ok := result["results"].([]any)
	assert.True(t, ok, "results should be an array")
	assert.Empty(t, results)
}

// ---------- ServeRAGMessage ----------

func TestServeRAGMessage_MethodNotAllowed(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodPost, "/api/rag/message?id=1", nil)
	w := callHandlerWithAuth(ServeRAGMessage, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestServeRAGMessage_MissingID(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/rag/message", nil)
	w := callHandlerWithAuth(ServeRAGMessage, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServeRAGMessage_InvalidID(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/rag/message?id=notanumber", nil)
	w := callHandlerWithAuth(ServeRAGMessage, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServeRAGMessage_NotFound(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/rag/message?id=99999", nil)
	w := callHandlerWithAuth(ServeRAGMessage, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestServeRAGMessage_Found(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	// Insert a message
	msgID, err := service.AddChatMessage(env.ProjectDir, "claude", "", "user", "hello", nil, false, "NewSession")
	require.NoError(t, err)

	req := newRequest(t, http.MethodGet, "/api/rag/message?id="+fmt.Sprint(msgID), nil)
	w := callHandlerWithAuth(ServeRAGMessage, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---------- ServeRAGSession ----------

func TestServeRAGSession_MissingID(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/rag/session", nil)
	w := callHandlerWithAuth(ServeRAGSession, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServeRAGSession_NotFound(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	// GetMessagesBySessionID returns empty slice (not error) for nonexistent session
	req := newRequest(t, http.MethodGet, "/api/rag/session?id=nonexistent", nil)
	w := callHandlerWithAuth(ServeRAGSession, req)
	// Returns 200 with empty messages array
	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "nonexistent", result["session_id"])
	msgs, ok := result["messages"].([]any)
	assert.True(t, ok)
	assert.Empty(t, msgs)
}

func TestServeRAGSession_Found(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	// Create a session and add messages
	sid, err := service.CreateSession(env.ProjectDir, "claude", "Test Session", "", "", "default")
	require.NoError(t, err)
	_, err = service.AddChatMessage(env.ProjectDir, "claude", sid, "user", "hello", nil, false, "NewSession")
	require.NoError(t, err)

	req := newRequest(t, http.MethodGet, "/api/rag/session?id="+sid, nil)
	w := callHandlerWithAuth(ServeRAGSession, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, sid, result["session_id"])
	msgs, ok := result["messages"].([]any)
	assert.True(t, ok, "messages should be an array")
	assert.NotEmpty(t, msgs)
}

// ---------- RAG test helpers ----------

// setupRAGStore creates a temporary DuckDB store for handler tests.
func setupRAGStore(t *testing.T) *rag.Store {
	t.Helper()
	dir := t.TempDir()
	store, err := rag.NewStore(dir + "/test.duckdb")
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return store
}

// setupMockEmbedder creates a mock EmbeddingClient pointing to a non-existent server.
func setupMockEmbedder(t *testing.T) *rag.EmbeddingClient {
	t.Helper()
	return rag.NewEmbeddingClient("http://127.0.0.1:1", "bge-m3")
}

// setupWorkingMockEmbedder creates a mock EmbeddingClient backed by a test server
// that returns valid 1024-dim embeddings.
func setupWorkingMockEmbedder(t *testing.T) *rag.EmbeddingClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/embeddings":
			// Return a 1024-dim embedding
			emb := make([]float64, 1024)
			for i := range emb {
				emb[i] = 0.01
			}
			json.NewEncoder(w).Encode(map[string]any{"embedding": emb})
		case "/api/tags":
			json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{{"name": "bge-m3:latest"}},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	client := rag.NewEmbeddingClient(server.URL, "bge-m3")
	client.HTTPClient = server.Client()
	return client
}
