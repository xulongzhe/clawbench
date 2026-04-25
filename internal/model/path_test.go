package model_test

import (
	"path/filepath"
	"testing"

	"clawbench/internal/model"

	"github.com/stretchr/testify/assert"
)

func TestValidatePath_ValidPath(t *testing.T) {
	base := filepath.Join(t.TempDir(), "base")
	path, valid := model.ValidatePath(base, "subdir/file.txt")
	assert.True(t, valid)
	assert.Contains(t, path, "subdir")
}

func TestValidatePath_BasePath(t *testing.T) {
	base := filepath.Join(t.TempDir(), "base")
	path, valid := model.ValidatePath(base, "")
	assert.True(t, valid)
	assert.Contains(t, path, "base")
}

func TestValidatePath_PathTraversal(t *testing.T) {
	base := filepath.Join(t.TempDir(), "base", "dir")
	path, valid := model.ValidatePath(base, "../../etc/passwd")
	assert.False(t, valid)
	_ = path // path is returned but not valid
}

func TestValidatePath_SimpleTraversal(t *testing.T) {
	base := filepath.Join(t.TempDir(), "base")
	_, valid := model.ValidatePath(base, "../outside")
	assert.False(t, valid)
}

func TestValidatePath_ValidSubdirectory(t *testing.T) {
	base := filepath.Join(t.TempDir(), "base")
	path, valid := model.ValidatePath(base, "sub/deep/file.go")
	assert.True(t, valid)
	assert.Contains(t, path, "sub")
}

func TestValidatePath_EmptyBaseAndRel(t *testing.T) {
	path, valid := model.ValidatePath("", "")
	assert.True(t, valid)
	// Should resolve to current working directory
	assert.NotEmpty(t, path)
}
