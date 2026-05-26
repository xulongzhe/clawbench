package model

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSummarizeConfig_IsChatSummaryEnabled_Nil(t *testing.T) {
	cfg := SummarizeConfig{}
	// ChatSummary is nil — should default to true
	assert.True(t, cfg.IsChatSummaryEnabled())
}

func TestSummarizeConfig_IsChatSummaryEnabled_True(t *testing.T) {
	val := true
	cfg := SummarizeConfig{ChatSummary: &val}
	assert.True(t, cfg.IsChatSummaryEnabled())
}

func TestSummarizeConfig_IsChatSummaryEnabled_False(t *testing.T) {
	val := false
	cfg := SummarizeConfig{ChatSummary: &val}
	assert.False(t, cfg.IsChatSummaryEnabled())
}

func TestIsSHA256Password_Plaintext(t *testing.T) {
	assert.False(t, IsSHA256Password("my-password"))
}

func TestIsSHA256Password_SHA256Prefix(t *testing.T) {
	assert.True(t, IsSHA256Password("sha256:abc123"))
}

func TestIsSHA256Password_Empty(t *testing.T) {
	assert.False(t, IsSHA256Password(""))
}

func TestParseSHA256Hash_Plaintext(t *testing.T) {
	assert.Equal(t, "", ParseSHA256Hash("my-password"))
}

func TestParseSHA256Hash_ValidSHA256(t *testing.T) {
	hash := "a" + string(make([]byte, 63)) // 64 chars
	// Use a proper 64-char hex string
	hash = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	result := ParseSHA256Hash("sha256:" + hash)
	assert.Equal(t, hash, result)
}

func TestParseSHA256Hash_InvalidTooShort(t *testing.T) {
	result := ParseSHA256Hash("sha256:abc")
	assert.Equal(t, "", result)
}

func TestParseSHA256Hash_InvalidTooLong(t *testing.T) {
	hash := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789extra"
	result := ParseSHA256Hash("sha256:" + hash)
	assert.Equal(t, "", result) // 64+5=69 chars, not 64
}

func TestApplyDefaults_SHA256PasswordRemovesAutoPasswordFile(t *testing.T) {
	tmpDir := t.TempDir()
	origBinDir := BinDir
	BinDir = tmpDir
	defer func() { BinDir = origBinDir }()

	// Create a stale auto-password file
	autoFile := filepath.Join(tmpDir, ".clawbench", "auto-password")
	require.NoError(t, os.MkdirAll(filepath.Dir(autoFile), 0755))
	require.NoError(t, os.WriteFile(autoFile, []byte("old-auto-password"), 0600))

	// Create a SHA-256 password
	hash := sha256.Sum256([]byte("test-password" + "clawbench-salt"))
	sha256Password := "sha256:" + hex.EncodeToString(hash[:])

	cfg := &Config{Password: sha256Password, WatchDir: tmpDir}
	ApplyDefaults(cfg, nil)

	// Auto-password file should be removed for SHA-256 stored password
	_, err := os.Stat(autoFile)
	assert.True(t, os.IsNotExist(err), "auto-password file should be removed when SHA-256 password is set")
}
