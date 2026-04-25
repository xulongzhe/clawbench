package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewBackend_Claude(t *testing.T) {
	backend, err := NewBackend("claude")
	assert.NoError(t, err)
	assert.NotNil(t, backend)
	assert.Equal(t, "claude", backend.Name())
}

func TestNewBackend_Codebuddy(t *testing.T) {
	backend, err := NewBackend("codebuddy")
	assert.NoError(t, err)
	assert.NotNil(t, backend)
	assert.Equal(t, "codebuddy", backend.Name())
}

func TestNewBackend_OpenCode(t *testing.T) {
	backend, err := NewBackend("opencode")
	assert.NoError(t, err)
	assert.NotNil(t, backend)
	assert.Equal(t, "opencode", backend.Name())
}

func TestNewBackend_Gemini(t *testing.T) {
	backend, err := NewBackend("gemini")
	assert.NoError(t, err)
	assert.NotNil(t, backend)
	assert.Equal(t, "gemini", backend.Name())
}

func TestNewBackend_Unsupported(t *testing.T) {
	_, err := NewBackend("unsupported")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported backend type")
}

func TestNewBackend_Empty(t *testing.T) {
	_, err := NewBackend("")
	assert.Error(t, err)
}
