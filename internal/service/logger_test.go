package service_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"clawbench/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFileHandler(t *testing.T) {
	dir := t.TempDir()
	h, err := service.NewFileHandler(dir, "test", 7)
	require.NoError(t, err)
	defer h.Close()

	// Check log file was created
	today := time.Now().Format("2006-01-02")
	expectedName := filepath.Join(dir, "test-"+today+".log")
	_, err = os.Stat(expectedName)
	assert.NoError(t, err)
}

func TestNewFileHandler_CreatesDir(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "logs", "subdir")
	h, err := service.NewFileHandler(dir, "app", 7)
	require.NoError(t, err)
	defer h.Close()

	info, err := os.Stat(dir)
	assert.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestFileHandler_Enabled(t *testing.T) {
	dir := t.TempDir()
	h, err := service.NewFileHandler(dir, "test", 7)
	require.NoError(t, err)
	defer h.Close()

	ctx := context.Background()
	assert.True(t, h.Enabled(ctx, slog.LevelError))
	assert.True(t, h.Enabled(ctx, slog.LevelInfo))
	assert.False(t, h.Enabled(ctx, slog.LevelDebug))
}

func TestFileHandler_Handle(t *testing.T) {
	dir := t.TempDir()
	h, err := service.NewFileHandler(dir, "test", 7)
	require.NoError(t, err)
	defer h.Close()

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
	err = h.Handle(context.Background(), record)
	assert.NoError(t, err)

	// Verify content was written
	today := time.Now().Format("2006-01-02")
	data, err := os.ReadFile(filepath.Join(dir, "test-"+today+".log"))
	assert.NoError(t, err)
	assert.Contains(t, string(data), "test message")
}

func TestFileHandler_Write(t *testing.T) {
	dir := t.TempDir()
	h, err := service.NewFileHandler(dir, "test", 7)
	require.NoError(t, err)
	defer h.Close()

	n, err := h.Write([]byte("hello world\n"))
	assert.NoError(t, err)
	assert.Equal(t, 12, n)

	// Verify content was written
	today := time.Now().Format("2006-01-02")
	data, err := os.ReadFile(filepath.Join(dir, "test-"+today+".log"))
	assert.NoError(t, err)
	assert.Contains(t, string(data), "hello world")
}

func TestFileHandler_WithGroup(t *testing.T) {
	dir := t.TempDir()
	h, err := service.NewFileHandler(dir, "test", 7)
	require.NoError(t, err)
	defer h.Close()

	h2 := h.WithGroup("mygroup")
	assert.NotNil(t, h2)
	// Should be a different handler instance
	assert.NotEqual(t, h, h2)
}

func TestFileHandler_WithAttrs(t *testing.T) {
	dir := t.TempDir()
	h, err := service.NewFileHandler(dir, "test", 7)
	require.NoError(t, err)
	defer h.Close()

	h2 := h.WithAttrs([]slog.Attr{slog.String("key", "value")})
	assert.NotNil(t, h2)
	assert.NotEqual(t, h, h2)
}

func TestFileHandler_Close(t *testing.T) {
	dir := t.TempDir()
	h, err := service.NewFileHandler(dir, "test", 7)
	require.NoError(t, err)

	err = h.Close()
	assert.NoError(t, err)

	// Double close returns "file already closed" error — expected behavior
	err = h.Close()
	assert.Error(t, err)
}

func TestFileHandler_CleanupOldLogs(t *testing.T) {
	dir := t.TempDir()

	// Create an old log file
	oldDate := time.Now().AddDate(0, 0, -10).Format("2006-01-02")
	oldFile := filepath.Join(dir, "test-"+oldDate+".log")
	err := os.WriteFile(oldFile, []byte("old log"), 0644)
	require.NoError(t, err)

	// Set modification time to make it old
	oldTime := time.Now().AddDate(0, 0, -10)
	os.Chtimes(oldFile, oldTime, oldTime)

	// Create a recent log file
	recentDate := time.Now().Format("2006-01-02")
	recentFile := filepath.Join(dir, "test-"+recentDate+".log")
	err = os.WriteFile(recentFile, []byte("recent log"), 0644)
	require.NoError(t, err)

	// NewFileHandler with maxDays=7 should clean up the old file
	h, err := service.NewFileHandler(dir, "test", 7)
	require.NoError(t, err)
	defer h.Close()

	// Old file should be removed
	_, err = os.Stat(oldFile)
	assert.True(t, os.IsNotExist(err), "old log file should be cleaned up")

	// Recent file should still exist
	_, err = os.Stat(recentFile)
	assert.NoError(t, err, "recent log file should not be cleaned up")
}

func TestFileHandler_MultipleHandle(t *testing.T) {
	dir := t.TempDir()
	h, err := service.NewFileHandler(dir, "test", 7)
	require.NoError(t, err)
	defer h.Close()

	for i := 0; i < 5; i++ {
		record := slog.NewRecord(time.Now(), slog.LevelInfo, "message "+string(rune('0'+i)), 0)
		err = h.Handle(context.Background(), record)
		assert.NoError(t, err)
	}

	today := time.Now().Format("2006-01-02")
	data, err := os.ReadFile(filepath.Join(dir, "test-"+today+".log"))
	assert.NoError(t, err)
	// Should contain all 5 messages
	for i := 0; i < 5; i++ {
		assert.Contains(t, string(data), "message "+string(rune('0'+i)))
	}
}

func TestFileHandler_MessagesPersistAfterClose(t *testing.T) {
	dir := t.TempDir()
	h, err := service.NewFileHandler(dir, "test", 7)
	require.NoError(t, err)

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "persist test", 0)
	h.Handle(context.Background(), record)
	h.Close()

	today := time.Now().Format("2006-01-02")
	data, err := os.ReadFile(filepath.Join(dir, "test-"+today+".log"))
	assert.NoError(t, err)
	assert.Contains(t, string(data), "persist test")
}
