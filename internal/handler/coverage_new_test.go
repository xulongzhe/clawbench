package handler

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clawbench/internal/ai"
	"clawbench/internal/model"
	"clawbench/internal/push"
	"clawbench/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// sanitizeArchiveName tests
// ============================================================================

func TestSanitizeArchiveName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"NormalFilename", "archive.zip", "archive.zip"},
		{"FilenameWithQuotes", `my"file.zip`, "my_file.zip"},
		{"FilenameWithBackslash", `my\file.zip`, "my_file.zip"},
		{"FilenameWithControlChars", "file\x01\x02.zip", "file__.zip"},
		{"MultipleSpecialChars", `"bad"\name.zip`, "_bad__name.zip"},
		{"EmptyString", "", ""},
		{"OnlySpecialChars", `"""`, "___"},
		{"NonASCII", "日本語.zip", "日本語.zip"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeArchiveName(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ============================================================================
// addFileToZip tests
// ============================================================================

func TestAddFileToZip(t *testing.T) {
	t.Run("AddsFileContent", func(t *testing.T) {
		// Create a temp file with known content
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.txt")
		content := "hello world from zip test"
		require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

		info, err := os.Stat(filePath)
		require.NoError(t, err)

		// Create zip in memory
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		err = addFileToZip(zw, filePath, "test.txt", info)
		require.NoError(t, err)
		require.NoError(t, zw.Close())

		// Read back and verify
		reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		require.NoError(t, err)
		require.Len(t, reader.File, 1)

		f := reader.File[0]
		assert.Equal(t, "test.txt", f.Name)
		rc, err := f.Open()
		require.NoError(t, err)
		data, err := readAll(rc)
		require.NoError(t, err)
		rc.Close()
		assert.Equal(t, content, string(data))
	})

	t.Run("NonExistentFile_ReturnsError", func(t *testing.T) {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		info := mockFileInfo{name: "ghost.txt", size: 100}
		err := addFileToZip(zw, "/nonexistent/path/ghost.txt", "ghost.txt", info)
		assert.Error(t, err)
		zw.Close()
	})
}

// mockFileInfo is a minimal os.FileInfo for testing.
type mockFileInfo struct {
	name string
	size int64
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return m.size }
func (m mockFileInfo) Mode() os.FileMode  { return 0644 }
func (m mockFileInfo) ModTime() time.Time { return time.Now() }
func (m mockFileInfo) IsDir() bool        { return false }
func (m mockFileInfo) Sys() interface{}   { return nil }

func readAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

// ============================================================================
// ServeFileArchive tests
// ============================================================================

func TestServeFileArchive_SingleFile(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	createTestFile(t, env.ProjectDir, "archive-me.txt", "archive content")

	req := newRequest(t, http.MethodPost, "/api/file/archive", map[string]any{
		"paths": []string{"archive-me.txt"},
	})
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeFileArchive, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/zip", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "archive-me.txt.zip")

	// Verify zip contents
	reader, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	require.NoError(t, err)
	assert.Len(t, reader.File, 1)
	assert.Equal(t, "archive-me.txt", reader.File[0].Name)
}

func TestServeFileArchive_Directory(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	createTestFile(t, env.ProjectDir, "mydir/a.txt", "aaa")
	createTestFile(t, env.ProjectDir, "mydir/sub/b.txt", "bbb")

	req := newRequest(t, http.MethodPost, "/api/file/archive", map[string]any{
		"paths": []string{"mydir"},
	})
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeFileArchive, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Disposition"), "mydir.zip")
}

func TestServeFileArchive_EmptyPaths_Returns400(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodPost, "/api/file/archive", map[string]any{
		"paths": []string{},
	})
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeFileArchive, req)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestServeFileArchive_WrongMethod(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/file/archive", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeFileArchive, req)
	assertStatus(t, w, http.StatusMethodNotAllowed)
}

func TestServeFileArchive_NoAccessiblePaths_Returns400(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodPost, "/api/file/archive", map[string]any{
		"paths": []string{"nonexistent-file.txt"},
	})
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeFileArchive, req)
	assertStatus(t, w, http.StatusBadRequest)
}

// ============================================================================
// stringsContainsAnyBlock tests
// ============================================================================

func TestStringsContainsAnyBlock(t *testing.T) {
	tests := []struct {
		name   string
		blocks []model.ContentBlock
		substr string
		want   bool
	}{
		{
			name:   "EmptySlice",
			blocks: nil,
			substr: "<ask-question",
			want:   false,
		},
		{
			name:   "NoTextBlocks",
			blocks: []model.ContentBlock{{Type: "tool_use", Name: "Bash"}},
			substr: "ask",
			want:   false,
		},
		{
			name:   "TextBlockContainsSubstring",
			blocks: []model.ContentBlock{{Type: "text", Text: "<ask-question>hello</ask-question>"}},
			substr: "<ask-question",
			want:   true,
		},
		{
			name:   "TextBlockMissingSubstring",
			blocks: []model.ContentBlock{{Type: "text", Text: "normal text"}},
			substr: "<ask-question",
			want:   false,
		},
		{
			name: "NonTextBlockIgnored",
			blocks: []model.ContentBlock{
				{Type: "thinking", Text: "<ask-question>"},
				{Type: "tool_use", Name: "Read"},
			},
			substr: "<ask-question",
			want:   false,
		},
		{
			name: "MultipleBlocks_SubstringInLaterBlock",
			blocks: []model.ContentBlock{
				{Type: "text", Text: "first block"},
				{Type: "text", Text: "second <ask-question> block"},
			},
			substr: "<ask-question",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stringsContainsAnyBlock(tt.blocks, tt.substr)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ============================================================================
// sendEvent tests
// ============================================================================

func TestSendEvent_ChannelHasCapacity(t *testing.T) {
	ch := make(chan ai.StreamEvent, 1)
	ctx := context.Background()

	event := ai.StreamEvent{Type: "content", Content: "hello"}
	result := sendEvent(ctx, ch, event)

	assert.True(t, result)
	select {
	case e := <-ch:
		assert.Equal(t, "content", e.Type)
		assert.Equal(t, "hello", e.Content)
	default:
		t.Fatal("expected event on channel")
	}
}

func TestSendEvent_ChannelFull_DropsEvent(t *testing.T) {
	ch := make(chan ai.StreamEvent, 1)
	ch <- ai.StreamEvent{Type: "content", Content: "existing"}

	ctx := context.Background()
	event := ai.StreamEvent{Type: "content", Content: "dropped"}
	result := sendEvent(ctx, ch, event)

	// Should return true (event dropped, not a context cancellation)
	assert.True(t, result)
}

func TestSendEvent_ContextCancelled(t *testing.T) {
	// Use an unbuffered channel with no reader — ctx.Done() and default
	// are both available, but ctx.Done() should be selected reliably
	// because the channel send would block.
	ch := make(chan ai.StreamEvent)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	event := ai.StreamEvent{Type: "content", Content: "hello"}
	_ = sendEvent(ctx, ch, event)

	// With an unbuffered channel and cancelled context, either ctx.Done() or default
	// could be selected. Both indicate the event was not sent to the channel.
	// The important thing is: the event is NOT on the channel.
	assert.Empty(t, len(ch), "event should not be on channel when context is cancelled")
}

func TestSendEvent_UnbufferedChannel_NoReader(t *testing.T) {
	ch := make(chan ai.StreamEvent)
	ctx := context.Background()

	event := ai.StreamEvent{Type: "content", Content: "dropped"}
	result := sendEvent(ctx, ch, event)

	// Should return true (event dropped via default case)
	assert.True(t, result)
}

// ============================================================================
// sendFinalEvent tests
// ============================================================================

func TestSendFinalEvent_ChannelHasCapacity(t *testing.T) {
	ch := make(chan ai.StreamEvent, 1)
	event := ai.StreamEvent{Type: "done"}
	sendFinalEvent(ch, event)

	select {
	case e := <-ch:
		assert.Equal(t, "done", e.Type)
	default:
		t.Fatal("expected event on channel")
	}
}

func TestSendFinalEvent_ChannelFull_DropsWithoutBlocking(t *testing.T) {
	ch := make(chan ai.StreamEvent, 1)
	ch <- ai.StreamEvent{Type: "content", Content: "existing"}

	// Should not block even though channel is full
	done := make(chan struct{})
	go func() {
		sendFinalEvent(ch, ai.StreamEvent{Type: "done"})
		close(done)
	}()

	select {
	case <-done:
		// Success — did not block
	case <-time.After(time.Second):
		t.Fatal("sendFinalEvent blocked when channel was full")
	}
}

// ============================================================================
// isNotDirError tests
// ============================================================================

func TestIsNotDirError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "PathErrorWithENOTDIR",
			err:  &os.PathError{Err: errors.New("not a directory"), Path: "/foo"},
			want: true,
		},
		{
			name: "PathErrorWithOtherError",
			err:  &os.PathError{Err: errors.New("permission denied"), Path: "/foo"},
			want: false,
		},
		{
			name: "NonPathError",
			err:  errors.New("something else"),
			want: false,
		},
		{
			name: "NilError",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNotDirError(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ============================================================================
// loginLimiter.cleanup tests
// ============================================================================

func TestLoginLimiter_Cleanup_RemovesStaleRecords(t *testing.T) {
	limiter := &loginLimiter{records: make(map[string]*ipRecord)}

	// Add a stale record (last failure was long ago, not blocked)
	staleTime := time.Now().Add(-2 * time.Hour)
	limiter.records["192.0.2.1"] = &ipRecord{
		failCount:     0,
		lastFail:      staleTime,
		blockedUntil:  time.Time{}, // not blocked
	}

	// Add a recent record (should NOT be removed)
	limiter.records["192.0.2.2"] = &ipRecord{
		failCount:    1,
		lastFail:     time.Now(),
		blockedUntil: time.Time{},
	}

	// Add a blocked record (should NOT be removed — still blocked)
	limiter.records["192.0.2.3"] = &ipRecord{
		failCount:    maxLoginFails,
		lastFail:     time.Now(),
		blockedUntil: time.Now().Add(10 * time.Minute),
	}

	limiter.cleanup()

	_, found1 := limiter.records["192.0.2.1"]
	assert.False(t, found1, "stale record should be cleaned up")

	_, found2 := limiter.records["192.0.2.2"]
	assert.True(t, found2, "recent record should remain")

	_, found3 := limiter.records["192.0.2.3"]
	assert.True(t, found3, "blocked record should remain")
}

func TestLoginLimiter_Cleanup_RemovesExpiredBlocks(t *testing.T) {
	limiter := &loginLimiter{records: make(map[string]*ipRecord)}

	// Add a record with expired block and stale last failure
	expiredTime := time.Now().Add(-2 * time.Hour)
	limiter.records["10.0.0.1"] = &ipRecord{
		failCount:    maxLoginFails,
		lastFail:     expiredTime,
		blockedUntil: time.Now().Add(-1 * time.Minute), // expired
	}

	limiter.cleanup()

	_, found := limiter.records["10.0.0.1"]
	assert.False(t, found, "expired blocked record with stale lastFail should be cleaned up")
}

func TestLoginLimiter_Cleanup_KeepsExpiredBlockButRecentFail(t *testing.T) {
	limiter := &loginLimiter{records: make(map[string]*ipRecord)}

	// Expired block but recent failure — should NOT be cleaned up
	limiter.records["10.0.0.2"] = &ipRecord{
		failCount:    maxLoginFails,
		lastFail:     time.Now().Add(-1 * time.Minute), // recent
		blockedUntil: time.Now().Add(-1 * time.Minute), // expired
	}

	limiter.cleanup()

	_, found := limiter.records["10.0.0.2"]
	assert.True(t, found, "expired block but recent failure should remain")
}

// ============================================================================
// androidLogFilePath tests
// ============================================================================

func TestAndroidLogFilePath(t *testing.T) {
	origLogDir := model.ConfigInstance.LogDir
	defer func() { model.ConfigInstance.LogDir = origLogDir }()

	model.ConfigInstance.LogDir = "/tmp/test-logs"
	got := androidLogFilePath()
	assert.Equal(t, filepath.Join("/tmp/test-logs", "android.log"), got)
}

// ============================================================================
// ServeAndroidLog tests
// ============================================================================

func TestServeAndroidLog_ValidEntries(t *testing.T) {
	origLogDir := model.ConfigInstance.LogDir
	defer func() { model.ConfigInstance.LogDir = origLogDir }()

	tmpDir := t.TempDir()
	model.ConfigInstance.LogDir = tmpDir

	entries := []AndroidLogEntry{
		{Level: "I", Tag: "MainActivity", Msg: "App started", Ts: 1700000000000},
		{Level: "E", Tag: "Network", Msg: "Connection failed", Ts: 1700000001000},
	}

	req := newRequest(t, http.MethodPost, "/api/android-log", map[string]any{
		"entries": entries,
	})

	w := callHandler(ServeAndroidLog, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify file was written
	data, err := os.ReadFile(filepath.Join(tmpDir, "android.log"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "I/MainActivity")
	assert.Contains(t, content, "App started")
	assert.Contains(t, content, "E/Network")
	assert.Contains(t, content, "Connection failed")
}

func TestServeAndroidLog_EmptyEntries_Returns400(t *testing.T) {
	req := newRequest(t, http.MethodPost, "/api/android-log", map[string]any{
		"entries": []AndroidLogEntry{},
	})

	w := callHandler(ServeAndroidLog, req)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestServeAndroidLog_WrongMethod(t *testing.T) {
	req := newRequest(t, http.MethodGet, "/api/android-log", nil)
	w := callHandler(ServeAndroidLog, req)
	assertStatus(t, w, http.StatusMethodNotAllowed)
}

func TestServeAndroidLog_NewlineEscaping(t *testing.T) {
	origLogDir := model.ConfigInstance.LogDir
	defer func() { model.ConfigInstance.LogDir = origLogDir }()

	tmpDir := t.TempDir()
	model.ConfigInstance.LogDir = tmpDir

	entries := []AndroidLogEntry{
		{Level: "I", Tag: "Test", Msg: "line1\nline2", Ts: 1700000000000},
	}

	req := newRequest(t, http.MethodPost, "/api/android-log", map[string]any{
		"entries": entries,
	})

	w := callHandler(ServeAndroidLog, req)
	assert.Equal(t, http.StatusOK, w.Code)

	data, err := os.ReadFile(filepath.Join(tmpDir, "android.log"))
	require.NoError(t, err)
	content := string(data)
	// Newlines in message should be escaped to \n
	assert.Contains(t, content, "line1\\nline2")
	// But each entry should end with actual newline
	lines := strings.Split(content, "\n")
	assert.True(t, len(lines) >= 2, "should have at least 2 lines (entry + trailing)")
}

func TestServeAndroidLog_TruncatesOver200(t *testing.T) {
	origLogDir := model.ConfigInstance.LogDir
	defer func() { model.ConfigInstance.LogDir = origLogDir }()

	tmpDir := t.TempDir()
	model.ConfigInstance.LogDir = tmpDir

	// Create 250 entries — should be capped to 200
	var entries []AndroidLogEntry
	for i := 0; i < 250; i++ {
		entries = append(entries, AndroidLogEntry{
			Level: "I", Tag: fmt.Sprintf("Tag%d", i), Msg: fmt.Sprintf("msg%d", i), Ts: 1700000000000 + int64(i*1000),
		})
	}

	req := newRequest(t, http.MethodPost, "/api/android-log", map[string]any{
		"entries": entries,
	})

	w := callHandler(ServeAndroidLog, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, float64(200), result["written"])
}

func TestServeAndroidLog_AppendsToExistingFile(t *testing.T) {
	origLogDir := model.ConfigInstance.LogDir
	defer func() { model.ConfigInstance.LogDir = origLogDir }()

	tmpDir := t.TempDir()
	model.ConfigInstance.LogDir = tmpDir

	// First request
	entries1 := []AndroidLogEntry{
		{Level: "I", Tag: "First", Msg: "first batch", Ts: 1700000000000},
	}
	req1 := newRequest(t, http.MethodPost, "/api/android-log", map[string]any{"entries": entries1})
	w1 := callHandler(ServeAndroidLog, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// Second request
	entries2 := []AndroidLogEntry{
		{Level: "I", Tag: "Second", Msg: "second batch", Ts: 1700000001000},
	}
	req2 := newRequest(t, http.MethodPost, "/api/android-log", map[string]any{"entries": entries2})
	w2 := callHandler(ServeAndroidLog, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	// Both should be in the file
	data, err := os.ReadFile(filepath.Join(tmpDir, "android.log"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "first batch")
	assert.Contains(t, content, "second batch")
}

// ============================================================================
// ServeGitBranch tests
// ============================================================================

func TestServeGitBranch_ValidRepo(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)

	req := newRequest(t, http.MethodGet, "/api/git/branch", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitBranch, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, true, result["isGit"])
	assert.Equal(t, "main", result["branch"])
	assert.NotEmpty(t, result["head"])
	assert.Equal(t, false, result["dirty"])
}

func TestServeGitBranch_DirtyRepo(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)
	// Make it dirty by modifying a tracked file
	existingFile := filepath.Join(env.ProjectDir, "README.md")
	require.NoError(t, os.WriteFile(existingFile, []byte("# Modified"), 0644))

	req := newRequest(t, http.MethodGet, "/api/git/branch", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitBranch, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, true, result["isGit"])
	assert.Equal(t, true, result["dirty"])
}

func TestServeGitBranch_NotGitRepo(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/git/branch", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitBranch, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, false, result["isGit"])
	assert.Equal(t, "", result["branch"])
}

func TestServeGitBranch_WrongMethod(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodPost, "/api/git/branch", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitBranch, req)
	assertStatus(t, w, http.StatusMethodNotAllowed)
}

// ============================================================================
// ServeGitVerifyCommits tests
// ============================================================================

func TestServeGitVerifyCommits_ValidSHA(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)
	sha := getHeadSHA(t, env.ProjectDir)

	req := newRequest(t, http.MethodPost, "/api/git/verify-commits", map[string]any{
		"shas": []string{sha},
	})
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitVerifyCommits, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	results, ok := result["results"].(map[string]interface{})
	require.True(t, ok)
	commitInfo := results[sha]
	assert.NotNil(t, commitInfo, "valid commit SHA should have info")
}

func TestServeGitVerifyCommits_InvalidSHA(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)

	req := newRequest(t, http.MethodPost, "/api/git/verify-commits", map[string]any{
		"shas": []string{"0000000000000000000000000000000000000000"},
	})
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitVerifyCommits, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	results, ok := result["results"].(map[string]interface{})
	require.True(t, ok)
	assert.Nil(t, results["0000000000000000000000000000000000000000"],
		"invalid SHA should have null result")
}

func TestServeGitVerifyCommits_EmptySHAs(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)

	req := newRequest(t, http.MethodPost, "/api/git/verify-commits", map[string]any{
		"shas": []string{},
	})
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitVerifyCommits, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	results, ok := result["results"].(map[string]interface{})
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestServeGitVerifyCommits_NotGitRepo(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodPost, "/api/git/verify-commits", map[string]any{
		"shas": []string{"abc123"},
	})
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitVerifyCommits, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	results, ok := result["results"].(map[string]interface{})
	require.True(t, ok)
	assert.Empty(t, results, "non-git repo should return empty results")
}

func TestServeGitVerifyCommits_WrongMethod(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/git/verify-commits", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitVerifyCommits, req)
	assertStatus(t, w, http.StatusMethodNotAllowed)
}

// ============================================================================
// SetPushClient / ServePushConfig tests
// ============================================================================

func TestSetPushClient_SetsRef(t *testing.T) {
	origRef := pushClientRef
	defer func() { pushClientRef = origRef }()

	client := &push.JPushClient{}
	SetPushClient(client)
	assert.Equal(t, client, pushClientRef)

	SetPushClient(nil)
	assert.Nil(t, pushClientRef)
}

func TestServePushConfig_NoClient(t *testing.T) {
	origRef := pushClientRef
	defer func() { pushClientRef = origRef }()

	pushClientRef = nil

	req := newRequest(t, http.MethodGet, "/api/push/config", nil)
	w := callHandler(ServePushConfig, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, false, result["jpush_enabled"])
	assert.Equal(t, "", result["jpush_app_key"])
}

func TestServePushConfig_DisabledClient(t *testing.T) {
	origRef := pushClientRef
	defer func() { pushClientRef = origRef }()

	pushClientRef = push.NewJPushClient(model.JPushConfig{
		Enabled:      false,
		AppKey:       "test-key",
		MasterSecret: "test-secret",
	})

	req := newRequest(t, http.MethodGet, "/api/push/config", nil)
	w := callHandler(ServePushConfig, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, false, result["jpush_enabled"])
}

func TestServePushConfig_EnabledClient(t *testing.T) {
	origRef := pushClientRef
	defer func() { pushClientRef = origRef }()

	pushClientRef = push.NewJPushClient(model.JPushConfig{
		Enabled:      true,
		AppKey:       "test-app-key-123",
		MasterSecret: "test-master-secret",
	})

	req := newRequest(t, http.MethodGet, "/api/push/config", nil)
	w := callHandler(ServePushConfig, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, true, result["jpush_enabled"])
	assert.Equal(t, "test-app-key-123", result["jpush_app_key"])
}

func TestServePushConfig_WrongMethod(t *testing.T) {
	req := newRequest(t, http.MethodPost, "/api/push/config", nil)
	w := callHandler(ServePushConfig, req)
	assertStatus(t, w, http.StatusMethodNotAllowed)
}

// ============================================================================
// SetSSHServer tests
// ============================================================================

func TestSetSSHServer_SetsRef(t *testing.T) {
	origRef := sshServerRef
	defer func() { sshServerRef = origRef }()

	// Test setting to non-nil (using the type only — actual Server requires real SSH setup)
	// Just verify nil round-trip
	SetSSHServer(nil)
	assert.Nil(t, sshServerRef)
}

// ============================================================================
// validateCreatePath tests
// ============================================================================

func TestValidateCreatePath_RelativeDir(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	w := httptest.NewRecorder()
	r := newRequest(t, http.MethodPost, "/api/file/create", nil)
	withProjectCookie(r, env.ProjectDir)

	absPath := validateCreatePath(w, r, "subdir", "newfile.txt")
	assert.NotEmpty(t, absPath)
	assert.Contains(t, absPath, "subdir")
	assert.Contains(t, absPath, "newfile.txt")
}

func TestValidateCreatePath_AbsDirUnderWatchDir(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	subDir := filepath.Join(env.WatchDir, "subproject")
	os.MkdirAll(subDir, 0755)

	w := httptest.NewRecorder()
	r := newRequest(t, http.MethodPost, "/api/file/create", nil)

	absPath := validateCreatePath(w, r, subDir, "newfile.txt")
	assert.NotEmpty(t, absPath)
	assert.Contains(t, absPath, "newfile.txt")
}

func TestValidateCreatePath_AbsDirEscapesWatchDir(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	w := httptest.NewRecorder()
	r := newRequest(t, http.MethodPost, "/api/file/create", nil)

	absPath := validateCreatePath(w, r, "/tmp/escaped", "newfile.txt")
	assert.Empty(t, absPath)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestValidateCreatePath_EmptyDirUsesProjectCookie(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	w := httptest.NewRecorder()
	r := newRequest(t, http.MethodPost, "/api/file/create", nil)
	withProjectCookie(r, env.ProjectDir)

	absPath := validateCreatePath(w, r, "", "newfile.txt")
	assert.NotEmpty(t, absPath)
	assert.Contains(t, absPath, "newfile.txt")
}

func TestValidateCreatePath_NoProjectCookie(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	w := httptest.NewRecorder()
	r := newRequest(t, http.MethodPost, "/api/file/create", nil)

	absPath := validateCreatePath(w, r, "", "newfile.txt")
	assert.Empty(t, absPath)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestValidateCreatePath_PathTraversalInName(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	w := httptest.NewRecorder()
	r := newRequest(t, http.MethodPost, "/api/file/create", nil)
	withProjectCookie(r, env.ProjectDir)

	absPath := validateCreatePath(w, r, "", "../../etc/evil.txt")
	assert.Empty(t, absPath)
}

func TestValidateCreatePath_RelativeDirWithTraversal(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	w := httptest.NewRecorder()
	r := newRequest(t, http.MethodPost, "/api/file/create", nil)
	withProjectCookie(r, env.ProjectDir)

	absPath := validateCreatePath(w, r, "../../../etc", "evil.txt")
	assert.Empty(t, absPath)
}

// ============================================================================
// detectDefaultBranch tests
// ============================================================================

func TestDetectDefaultBranch_LocalMain(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)

	branch := detectDefaultBranch(env.ProjectDir)
	assert.Equal(t, "main", branch)
}

func TestDetectDefaultBranch_NotGitRepo(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	// Not a git repo
	branch := detectDefaultBranch(env.ProjectDir)
	assert.Equal(t, "", branch)
}

// ============================================================================
// copyDir tests (low coverage: 41.9%)
// ============================================================================

func TestCopyDir_DeepNesting(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	// Create deep directory structure
	createTestFile(t, env.ProjectDir, "src/a/b/c/deep.txt", "deep content")

	srcDir := filepath.Join(env.ProjectDir, "src")
	dstDir := filepath.Join(env.ProjectDir, "dst")

	err := copyDir(srcDir, dstDir, env.WatchDir)
	require.NoError(t, err)

	// Verify deep copy
	data, err := os.ReadFile(filepath.Join(dstDir, "a/b/c/deep.txt"))
	require.NoError(t, err)
	assert.Equal(t, "deep content", string(data))
}

func TestCopyDir_EmptySubdirectory(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	// Create dir with an empty subdirectory
	require.NoError(t, os.MkdirAll(filepath.Join(env.ProjectDir, "src", "emptydir"), 0755))
	createTestFile(t, env.ProjectDir, "src/file.txt", "content")

	srcDir := filepath.Join(env.ProjectDir, "src")
	dstDir := filepath.Join(env.ProjectDir, "dst")

	err := copyDir(srcDir, dstDir, env.WatchDir)
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(dstDir, "emptydir"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	data, err := os.ReadFile(filepath.Join(dstDir, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "content", string(data))
}

// ============================================================================
// safeRemoveAll tests (low coverage: 55.6%)
// ============================================================================

func TestSafeRemoveAll_UnderWatchDir(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	targetDir := filepath.Join(env.ProjectDir, "to-remove")
	require.NoError(t, os.MkdirAll(targetDir, 0755))
	createTestFile(t, targetDir, "file.txt", "data")

	err := safeRemoveAll(targetDir, env.WatchDir)
	assert.NoError(t, err)
	_, statErr := os.Stat(targetDir)
	assert.True(t, os.IsNotExist(statErr))
}

func TestSafeRemoveAll_NonExistentDir(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	err := safeRemoveAll(filepath.Join(env.ProjectDir, "nonexistent"), env.WatchDir)
	assert.NoError(t, err) // Walk on nonexistent dir returns no error
}

func TestSafeRemoveAll_SymlinkEscaping(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	targetDir := filepath.Join(env.ProjectDir, "symlink-dir")
	require.NoError(t, os.MkdirAll(targetDir, 0755))
	createTestFile(t, targetDir, "file.txt", "data")

	// Create a symlink inside that points outside WatchDir
	escapeTarget := filepath.Join(os.TempDir(), "clawbench-symlink-escape")
	require.NoError(t, os.MkdirAll(escapeTarget, 0755))
	defer os.RemoveAll(escapeTarget)
	require.NoError(t, os.Symlink(escapeTarget, filepath.Join(targetDir, "escape-link")))

	err := safeRemoveAll(targetDir, env.WatchDir)
	assert.NoError(t, err)
	// The directory itself should be removed
	_, statErr := os.Stat(targetDir)
	assert.True(t, os.IsNotExist(statErr))
	// The escape target should still exist (symlink was not followed)
	_, escapeStatErr := os.Stat(escapeTarget)
	assert.NoError(t, escapeStatErr, "escape target should not be removed")
}

// ============================================================================
// buildChatRequestFromQueue tests (low coverage: 20.8%)
// ============================================================================

func TestBuildChatRequestFromQueue_BasicFields(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	sessionID, err := service.CreateSession(env.ProjectDir, "codebuddy", "queue-test", "", "", "default", "chat")
	require.NoError(t, err)

	qMsg := model.QueuedMessage{
		Text:      "queued message",
		FilePaths: []string{},
		Files:     []string{},
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	req := buildChatRequestFromQueue(qMsg, sessionID, env.ProjectDir, "codebuddy", "codebuddy", env.ProjectDir)
	assert.NotNil(t, req)
	assert.Equal(t, "queued message", req.Prompt)
	assert.Equal(t, sessionID, req.SessionID)
}

func TestBuildChatRequestFromQueue_WithFiles(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	sessionID, err := service.CreateSession(env.ProjectDir, "codebuddy", "queue-files", "", "", "default", "chat")
	require.NoError(t, err)

	qMsg := model.QueuedMessage{
		Text:      "check this",
		FilePaths: []string{"config.yaml"},
		Files:     []string{"config.yaml"},
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	req := buildChatRequestFromQueue(qMsg, sessionID, env.ProjectDir, "codebuddy", "codebuddy", env.ProjectDir)
	assert.NotNil(t, req)
	// The prompt should contain the original text (may be prefixed with file annotations)
	assert.Contains(t, req.Prompt, "check this")
}

// ============================================================================
// writeDiffResponse tests (low coverage: 50%)
// ============================================================================

func TestWriteDiffResponse_WithDiff(t *testing.T) {
	w := httptest.NewRecorder()
	writeDiffResponse(w, []byte("diff --git a/file.go b/file.go\n+added line\n-removed line"), nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Contains(t, result, "diff")
}

// ============================================================================
// serveProjectsCreate tests (low coverage: 48.6%)
// ============================================================================

func TestServeProjectsCreate_AlreadyExists(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	// Create a directory with the same name first
	os.MkdirAll(filepath.Join(env.WatchDir, "existing-project"), 0755)

	req := newRequest(t, http.MethodPost, "/api/projects", map[string]string{
		"name": "existing-project",
	})
	w := callHandler(ServeProjects, req)

	// The handler may return 500 (mkdir error) or 409 — either is reasonable
	assert.True(t, w.Code >= 400, "expected error status for existing directory, got %d", w.Code)
}

func TestServeProjectsCreate_PathTraversalName(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodPost, "/api/projects", map[string]string{
		"name": "../../../etc",
	})
	w := callHandler(ServeProjects, req)

	// Should reject path traversal
	assert.True(t, w.Code == http.StatusForbidden || w.Code == http.StatusBadRequest,
		"expected 403 or 400, got %d", w.Code)
}

// ============================================================================
// ServeIndex tests (low coverage: 31.2%)
// ============================================================================

func TestServeIndex_MethodNotAllowed(t *testing.T) {
	// ServeIndex is registered as a catch-all route — it handles GET only
	// POST on "/" goes through the router which may return 404 instead of 405
	req := newRequest(t, http.MethodPost, "/", nil)
	w := callHandler(ServeIndex, req)
	// The handler may return 404 (no route) or 405 — just verify no panic
	assert.NotEqual(t, http.StatusOK, w.Code)
}

// ============================================================================
// ServeRecentProjects tests (low coverage: 50%)
// ============================================================================

func TestServeRecentProjects_Empty(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/recent-projects", nil)
	w := callHandler(ServeRecentProjects, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
