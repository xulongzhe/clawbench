package model_test

import (
	"os"
	"path/filepath"
	"testing"

	"clawbench/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelCache_ReadWrite(t *testing.T) {
	dir := t.TempDir()

	// Cache is empty initially
	models := model.ReadModelCache(dir, "codebuddy")
	assert.Nil(t, models)

	// Write cache
	written := []model.AgentModel{
		{ID: "glm-5.1", Name: "GLM 5.1", Default: true},
		{ID: "glm-4.7", Name: "GLM 4.7", Default: false},
	}
	require.NoError(t, model.WriteModelCache(dir, "codebuddy", written))

	// Read back
	models = model.ReadModelCache(dir, "codebuddy")
	require.Len(t, models, 2)
	assert.Equal(t, "glm-5.1", models[0].ID)
	assert.True(t, models[0].Default)
	assert.Equal(t, "glm-4.7", models[1].ID)
}

func TestModelCache_CorruptFile(t *testing.T) {
	dir := t.TempDir()

	// Write garbage
	cachePath := filepath.Join(dir, "codebuddy.json")
	require.NoError(t, os.WriteFile(cachePath, []byte("not json"), 0o644))

	// Should return nil gracefully
	models := model.ReadModelCache(dir, "codebuddy")
	assert.Nil(t, models)
}

func TestModelCache_EmptyModels(t *testing.T) {
	dir := t.TempDir()

	// Write empty models list — should not create cache file
	err := model.WriteModelCache(dir, "test", nil)
	require.NoError(t, err)

	models := model.ReadModelCache(dir, "test")
	assert.Nil(t, models)
}

func TestModelCache_NonexistentDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "no-such-dir")

	// Read from nonexistent dir — should return nil
	models := model.ReadModelCache(dir, "codebuddy")
	assert.Nil(t, models)

	// Write creates the directory
	written := []model.AgentModel{
		{ID: "model-a", Name: "Model A", Default: true},
	}
	require.NoError(t, model.WriteModelCache(dir, "test", written))
	models = model.ReadModelCache(dir, "test")
	require.Len(t, models, 1)
	assert.Equal(t, "model-a", models[0].ID)
}

func TestModelCache_WriteMkdirAllError(t *testing.T) {
	// On non-Windows, creating a directory under a read-only parent should fail
	if os.PathSeparator == '\\' {
		t.Skip("read-only directory test not reliable on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("skipping: root user bypasses filesystem permissions")
	}

	parent := t.TempDir()
	// Make parent read-only so MkdirAll fails
	require.NoError(t, os.Chmod(parent, 0o555))
	defer os.Chmod(parent, 0o755) // restore for cleanup

	nestedDir := filepath.Join(parent, "sub", "cache")
	err := model.WriteModelCache(nestedDir, "test", []model.AgentModel{
		{ID: "model-a", Name: "Model A", Default: true},
	})
	assert.Error(t, err, "writing to read-only parent directory should fail")
}

func TestModelCache_ReadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	// Write JSON that has the right structure but models is wrong type
	cachePath := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(cachePath, []byte(`{"updated_at":"2024-01-01","models":"not an array"}`), 0o644))

	models := model.ReadModelCache(dir, "bad")
	assert.Nil(t, models, "should return nil for JSON with wrong models type")
}
