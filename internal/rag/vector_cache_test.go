package rag

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- VectorCache: CosineSimilarity ----------

func TestCosineSimilarity_IdenticalVectors(t *testing.T) {
	vec := []float64{1.0, 2.0, 3.0}
	score := cosineSimilarity(vec, vec)
	assert.InDelta(t, 1.0, score, 1e-9, "identical vectors should have similarity 1.0")
}

func TestCosineSimilarity_OrthogonalVectors(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{0, 1, 0}
	score := cosineSimilarity(a, b)
	assert.InDelta(t, 0.0, score, 1e-9, "orthogonal vectors should have similarity 0.0")
}

func TestCosineSimilarity_OppositeVectors(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{-1, 0, 0}
	score := cosineSimilarity(a, b)
	assert.InDelta(t, -1.0, score, 1e-9, "opposite vectors should have similarity -1.0")
}

func TestCosineSimilarity_DifferentMagnitudes(t *testing.T) {
	a := []float64{1, 2, 3}
	b := []float64{2, 4, 6} // same direction, different magnitude
	score := cosineSimilarity(a, b)
	assert.InDelta(t, 1.0, score, 1e-9, "same-direction vectors should have similarity 1.0")
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := []float64{0, 0, 0}
	b := []float64{1, 2, 3}
	score := cosineSimilarity(a, b)
	assert.InDelta(t, 0.0, score, 1e-9, "zero vector should produce 0 similarity due to epsilon guard")
}

// ---------- VectorCache: Search ----------

func TestVectorCache_Search_Empty(t *testing.T) {
	cache := NewVectorCache(0)
	hits := cache.Search([]float64{1, 2, 3}, 5, "", "", "", "", "")
	assert.Empty(t, hits, "empty cache should return no results")
}

func TestVectorCache_Search_Basic(t *testing.T) {
	cache := NewVectorCache(3)
	cache.SetVectors([]CachedVector{
		{ChunkID: 1, SessionID: "s1", ProjectPath: "/p1", Vector: []float64{1, 0, 0}},
		{ChunkID: 2, SessionID: "s2", ProjectPath: "/p2", Vector: []float64{0, 1, 0}},
		{ChunkID: 3, SessionID: "s3", ProjectPath: "/p1", Vector: []float64{0, 0, 1}},
	})

	// Search for vector closest to [1,0,0]
	hits := cache.Search([]float64{1, 0, 0}, 3, "", "", "", "", "")
	require.Len(t, hits, 3)
	assert.Equal(t, int64(1), hits[0].ChunkID, "closest vector should be chunk 1")
	assert.InDelta(t, 1.0, hits[0].Score, 1e-9)
}

func TestVectorCache_Search_RespectsLimit(t *testing.T) {
	cache := NewVectorCache(3)
	cache.SetVectors([]CachedVector{
		{ChunkID: 1, SessionID: "s1", ProjectPath: "/p1", Vector: []float64{1, 0, 0}},
		{ChunkID: 2, SessionID: "s2", ProjectPath: "/p2", Vector: []float64{0.9, 0.1, 0}},
		{ChunkID: 3, SessionID: "s3", ProjectPath: "/p1", Vector: []float64{0, 0, 1}},
	})

	hits := cache.Search([]float64{1, 0, 0}, 2, "", "", "", "", "")
	assert.LessOrEqual(t, len(hits), 2, "should respect limit")
}

func TestVectorCache_Search_FiltersByProjectPath(t *testing.T) {
	cache := NewVectorCache(3)
	cache.SetVectors([]CachedVector{
		{ChunkID: 1, SessionID: "s1", ProjectPath: "/p1", Vector: []float64{1, 0, 0}},
		{ChunkID: 2, SessionID: "s2", ProjectPath: "/p2", Vector: []float64{1, 0, 0}},
		{ChunkID: 3, SessionID: "s3", ProjectPath: "/p1", Vector: []float64{0.5, 0.5, 0}},
	})

	hits := cache.Search([]float64{1, 0, 0}, 10, "/p1", "", "", "", "")
	for _, h := range hits {
		assert.Equal(t, "/p1", h.ProjectPath, "should only return /p1 results")
	}
	assert.Len(t, hits, 2)
}

func TestVectorCache_Search_FiltersByBackend(t *testing.T) {
	cache := NewVectorCache(2)
	cache.SetVectors([]CachedVector{
		{ChunkID: 1, SessionID: "s1", ProjectPath: "/p", Backend: "claude", Vector: []float64{1, 0, 0}},
		{ChunkID: 2, SessionID: "s2", ProjectPath: "/p", Backend: "codebuddy", Vector: []float64{1, 0, 0}},
	})

	hits := cache.Search([]float64{1, 0, 0}, 10, "", "claude", "", "", "")
	require.Len(t, hits, 1)
	assert.Equal(t, "claude", hits[0].Backend)
}

func TestVectorCache_Search_FiltersByRole(t *testing.T) {
	cache := NewVectorCache(2)
	cache.SetVectors([]CachedVector{
		{ChunkID: 1, SessionID: "s1", ProjectPath: "/p", Role: "assistant", Vector: []float64{1, 0, 0}},
		{ChunkID: 2, SessionID: "s2", ProjectPath: "/p", Role: "user", Vector: []float64{1, 0, 0}},
	})

	hits := cache.Search([]float64{1, 0, 0}, 10, "", "", "user", "", "")
	require.Len(t, hits, 1)
	assert.Equal(t, "user", hits[0].Role)
}

func TestVectorCache_Search_FiltersBySessionID(t *testing.T) {
	cache := NewVectorCache(2)
	cache.SetVectors([]CachedVector{
		{ChunkID: 1, SessionID: "s-target", ProjectPath: "/p", Vector: []float64{1, 0, 0}},
		{ChunkID: 2, SessionID: "s-other", ProjectPath: "/p", Vector: []float64{1, 0, 0}},
	})

	hits := cache.Search([]float64{1, 0, 0}, 10, "", "", "", "s-target", "")
	require.Len(t, hits, 1)
	assert.Equal(t, "s-target", hits[0].SessionID)
}

func TestVectorCache_Search_ExcludeSessionID(t *testing.T) {
	cache := NewVectorCache(2)
	cache.SetVectors([]CachedVector{
		{ChunkID: 1, SessionID: "s-exclude", ProjectPath: "/p", Vector: []float64{1, 0, 0}},
		{ChunkID: 2, SessionID: "s-keep", ProjectPath: "/p", Vector: []float64{0.9, 0.1, 0}},
	})

	hits := cache.Search([]float64{1, 0, 0}, 10, "", "", "", "", "s-exclude")
	for _, h := range hits {
		assert.NotEqual(t, "s-exclude", h.SessionID)
	}
}

func TestVectorCache_Search_OrderByScore(t *testing.T) {
	cache := NewVectorCache(3)
	cache.SetVectors([]CachedVector{
		{ChunkID: 1, SessionID: "s1", ProjectPath: "/p", Vector: []float64{0.5, 0.5, 0}},
		{ChunkID: 2, SessionID: "s2", ProjectPath: "/p", Vector: []float64{1, 0, 0}},
		{ChunkID: 3, SessionID: "s3", ProjectPath: "/p", Vector: []float64{0.3, 0.3, 0.3}},
	})

	hits := cache.Search([]float64{1, 0, 0}, 10, "", "", "", "", "")
	for i := 1; i < len(hits); i++ {
		assert.GreaterOrEqual(t, hits[i-1].Score, hits[i].Score, "results should be ordered by score descending")
	}
}

// ---------- VectorCache: Ready gate ----------

func TestVectorCache_NotReady(t *testing.T) {
	cache := NewVectorCache(3)
	assert.False(t, cache.IsReady(), "new cache should not be ready")
}

func TestVectorCache_ReadyAfterSetVectors(t *testing.T) {
	cache := NewVectorCache(3)
	cache.SetVectors([]CachedVector{
		{ChunkID: 1, SessionID: "s1", ProjectPath: "/p", Vector: []float64{1, 0, 0}},
	})
	assert.True(t, cache.IsReady(), "cache should be ready after SetVectors")
}

func TestVectorCache_SearchReturnsEmptyWhenNotReady(t *testing.T) {
	cache := NewVectorCache(3)
	// Don't call SetVectors — cache is not ready
	hits := cache.Search([]float64{1, 0, 0}, 5, "", "", "", "", "")
	assert.Empty(t, hits, "should return empty when not ready")
}

// ---------- VectorCache: Dim ----------

func TestVectorCache_Dim(t *testing.T) {
	cache := NewVectorCache(0)
	assert.Equal(t, 0, cache.Dim())

	cache = NewVectorCache(1024)
	assert.Equal(t, 1024, cache.Dim())
}

// ---------- VectorCache: SetDim / Clear ----------

func TestVectorCache_SetDim(t *testing.T) {
	cache := NewVectorCache(0)
	cache.SetDim(768)
	assert.Equal(t, 768, cache.Dim())
}

func TestVectorCache_Clear(t *testing.T) {
	cache := NewVectorCache(1024)
	cache.SetVectors([]CachedVector{
		{ChunkID: 1, SessionID: "s1", ProjectPath: "/p", Vector: makeTestEmbedding(1024)},
	})
	assert.True(t, cache.IsReady())

	cache.Clear()
	assert.False(t, cache.IsReady(), "clear should mark cache as not ready")
	assert.Empty(t, cache.Search(makeTestEmbedding(1024), 5, "", "", "", "", ""))
}

// ---------- VectorCache: MarkDirty / LoadIncremental ----------

func TestVectorCache_MarkDirty(t *testing.T) {
	cache := NewVectorCache(3)
	assert.False(t, cache.IsDirty())

	cache.MarkDirty()
	assert.True(t, cache.IsDirty())
}

func TestVectorCache_MarkDirtyNotSetAfterSetVectors(t *testing.T) {
	cache := NewVectorCache(3)
	cache.MarkDirty()
	cache.SetVectors([]CachedVector{
		{ChunkID: 1, SessionID: "s1", ProjectPath: "/p", Vector: []float64{1, 0, 0}},
	})
	assert.False(t, cache.IsDirty(), "SetVectors should clear dirty flag")
}

// ---------- BLOB serialization ----------

func TestSerializeDeserializeEmbedding(t *testing.T) {
	original := []float64{1.0, -2.5, 3.14, 0.0, -0.001}
	blob := serializeEmbedding(original)
	restored := deserializeEmbedding(blob, len(original))

	assert.Equal(t, len(original), len(restored))
	for i := range original {
		assert.InDelta(t, original[i], restored[i], 1e-15, "round-trip should preserve values")
	}
}

func TestSerializeEmbedding_Size(t *testing.T) {
	vec := make([]float64, 1024)
	blob := serializeEmbedding(vec)
	assert.Equal(t, 1024*8, len(blob), "each float64 should be 8 bytes")
}

func TestDeserializeEmbedding_WrongSize(t *testing.T) {
	blob := make([]byte, 10) // not a multiple of 8
	vec := deserializeEmbedding(blob, 2)
	// Should handle gracefully — dimension mismatch means only 1 float64 fits
	assert.Len(t, vec, 1) // 10/8 = 1 with remainder
}

// ---------- cosineSimilarity edge cases ----------

func TestCosineSimilarity_LargeDimension(t *testing.T) {
	// Test with realistic embedding dimension (1024)
	a := makeTestEmbedding(1024)
	b := makeTestEmbedding(1024)
	score := cosineSimilarity(a, b)
	assert.InDelta(t, 1.0, score, 1e-6, "identical large vectors should have similarity ~1.0")
}

func TestCosineSimilarity_PartiallyOverlapping(t *testing.T) {
	// Two vectors with partial overlap — neither parallel nor orthogonal
	a := []float64{1.0, 0.5, 0.0}
	b := []float64{0.5, 1.0, 0.0}
	score := cosineSimilarity(a, b)
	assert.Greater(t, score, 0.0, "partially overlapping vectors should have positive similarity")
	assert.Less(t, score, 1.0, "non-parallel vectors should have similarity < 1.0")
}

// ---------- VectorCache: Search with NaN in query ----------

func TestVectorCache_Search_NanQueryEmbedding(t *testing.T) {
	cache := NewVectorCache(3)
	cache.SetVectors([]CachedVector{
		{ChunkID: 1, SessionID: "s1", ProjectPath: "/p", Vector: []float64{1, 0, 0}},
	})

	query := []float64{1, 0, math.NaN()}
	// Should not panic; NaN comparisons yield false scores, results may be empty or weird
	hits := cache.Search(query, 5, "", "", "", "", "")
	// Main assertion: no panic
	_ = hits
}
