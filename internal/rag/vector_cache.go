package rag

import (
	"math"
	"sort"
	"sync"
)

// CachedVector holds a deserialized embedding with its chunk metadata.
type CachedVector struct {
	ChunkID     int64
	SessionID   string
	ProjectPath string
	Backend     string
	Role        string
	Vector      []float64
}

// VectorCache holds all embeddings in memory for fast cosine similarity search.
// It is loaded asynchronously at startup; search returns empty results until ready.
type VectorCache struct {
	mu      sync.RWMutex
	vectors []CachedVector
	dim     int
	ready   bool
	dirty   bool
}

// NewVectorCache creates a VectorCache with the given embedding dimension.
func NewVectorCache(dim int) *VectorCache {
	return &VectorCache{dim: dim}
}

// IsReady returns whether the cache has been loaded and is ready for search.
func (c *VectorCache) IsReady() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ready
}

// IsDirty returns whether the cache needs incremental reload.
func (c *VectorCache) IsDirty() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dirty
}

// Dim returns the current embedding dimension.
func (c *VectorCache) Dim() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dim
}

// SetDim sets the embedding dimension.
func (c *VectorCache) SetDim(dim int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dim = dim
}

// SetVectors replaces the entire vector set. Marks cache as ready and clears dirty flag.
func (c *VectorCache) SetVectors(vectors []CachedVector) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.vectors = vectors
	c.ready = true
	c.dirty = false
}

// Clear removes all vectors and marks the cache as not ready.
func (c *VectorCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.vectors = nil
	c.ready = false
	c.dirty = false
}

// MarkDirty signals that new embeddings have been added and the cache needs reload.
func (c *VectorCache) MarkDirty() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dirty = true
}

// Search performs in-memory cosine similarity search against the cached vectors.
// Returns SearchHit results sorted by score descending, limited to `limit` results.
// If the cache is not ready, returns empty results.
// Filters: projectPath, backend, role, sessionID, excludeSessionID (empty = no filter).
func (c *VectorCache) Search(queryEmbedding []float64, limit int, projectPath, backend, role, sessionID, excludeSessionID string) []SearchHit {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.ready || len(c.vectors) == 0 {
		return nil
	}

	type scored struct {
		hit   SearchHit
		score float64
	}

	candidates := make([]scored, 0, len(c.vectors))
	for _, v := range c.vectors {
		// Apply filters
		if projectPath != "" && v.ProjectPath != projectPath {
			continue
		}
		if backend != "" && v.Backend != backend {
			continue
		}
		if role != "" && v.Role != role {
			continue
		}
		if sessionID != "" && v.SessionID != sessionID {
			continue
		}
		if excludeSessionID != "" && v.SessionID == excludeSessionID {
			continue
		}

		score := cosineSimilarity(queryEmbedding, v.Vector)
		if math.IsNaN(score) {
			continue
		}

		candidates = append(candidates, scored{
			hit: SearchHit{
				ChunkID:     v.ChunkID,
				SessionID:   v.SessionID,
				ProjectPath: v.ProjectPath,
				Backend:     v.Backend,
				Role:        v.Role,
				Score:       score,
			},
			score: score,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if limit > len(candidates) {
		limit = len(candidates)
	}

	results := make([]SearchHit, limit)
	for i, c := range candidates[:limit] {
		results[i] = c.hit
	}
	return results
}

// cosineSimilarity computes the cosine similarity between two vectors.
// Returns 0 if either vector has zero magnitude (epsilon guard).
func cosineSimilarity(a, b []float64) float64 {
	var dot, normA, normB float64
	n := len(a)
	if n > len(b) {
		n = len(b)
	}
	for i := range n {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom < 1e-8 {
		return 0.0
	}
	return dot / denom
}

// serializeEmbedding converts a []float64 to a byte slice for BLOB storage.
// Each float64 is stored as 8 bytes using math.Float64bits.
func serializeEmbedding(vec []float64) []byte {
	buf := make([]byte, len(vec)*8)
	for i, v := range vec {
		bits := math.Float64bits(v)
		buf[i*8+0] = byte(bits >> 56)
		buf[i*8+1] = byte(bits >> 48)
		buf[i*8+2] = byte(bits >> 40)
		buf[i*8+3] = byte(bits >> 32)
		buf[i*8+4] = byte(bits >> 24)
		buf[i*8+5] = byte(bits >> 16)
		buf[i*8+6] = byte(bits >> 8)
		buf[i*8+7] = byte(bits)
	}
	return buf
}

// deserializeEmbedding converts a BLOB byte slice back to []float64.
// dim specifies the expected number of float64 values.
func deserializeEmbedding(buf []byte, dim int) []float64 {
	vec := make([]float64, 0, dim)
	for i := 0; i+8 <= len(buf) && len(vec) < dim; i += 8 {
		bits := uint64(buf[i+0])<<56 |
			uint64(buf[i+1])<<48 |
			uint64(buf[i+2])<<40 |
			uint64(buf[i+3])<<32 |
			uint64(buf[i+4])<<24 |
			uint64(buf[i+5])<<16 |
			uint64(buf[i+6])<<8 |
			uint64(buf[i+7])
		vec = append(vec, math.Float64frombits(bits))
	}
	return vec
}
