package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"clawbench/internal/model"

	"github.com/stretchr/testify/assert"
)

// initGitRepo initializes a real git repo in dir with an initial commit.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %s %v failed: %v\n%s", name, args, err, out)
		}
	}
	run("git", "init")
	run("git", "config", "user.email", "test@test.com")
	run("git", "config", "user.name", "Test")

	// Create initial file and commit
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}
	run("git", "add", ".")
	run("git", "commit", "-m", "initial commit")
}

// gitCommitAll stages all changes and commits with the given message.
func gitCommitAll(t *testing.T, dir, message string) {
	t.Helper()
	run := func(name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("command %s %v failed: %v\n%s", name, args, err, out)
		}
	}
	run("git", "add", ".")
	run("git", "commit", "-m", message)
}

// getHeadSHA returns the SHA of the HEAD commit.
func getHeadSHA(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get HEAD SHA: %v", err)
	}
	return string(out[:len(out)-1]) // trim newline
}

// --- ServeGitInit ---

func TestServeGitInit_Success(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodPost, "/api/git/init", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitInit, req)
	assertOK(t, w)
	assertJSONField(t, w, "success", true)

	// Verify .git directory exists
	_, err := os.Stat(filepath.Join(env.ProjectDir, ".git"))
	assert.NoError(t, err)
}

func TestServeGitInit_AlreadyRepo(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)

	req := newRequest(t, http.MethodPost, "/api/git/init", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitInit, req)
	assertStatus(t, w, http.StatusBadRequest)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "already a git repository", resp["error"])
}

func TestServeGitInit_NoProjectCookie(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodPost, "/api/git/init", nil)
	// No project cookie

	w := callHandler(ServeGitInit, req)
	assertStatus(t, w, http.StatusForbidden)
}

func TestServeGitInit_WrongMethod(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/git/init", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitInit, req)
	assertStatus(t, w, http.StatusMethodNotAllowed)
}

// --- ServeGitStatus ---

func TestServeGitStatus_UncommittedChanges(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)

	// Modify a file to create uncommitted changes
	createTestFile(t, env.ProjectDir, "README.md", "# Modified")

	req := newRequest(t, http.MethodGet, "/api/git/status?path=README.md", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitStatus, req)
	assertOK(t, w)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["isGit"])
	assert.Equal(t, true, resp["hasUncommitted"])
}

func TestServeGitStatus_NoChanges(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)

	req := newRequest(t, http.MethodGet, "/api/git/status?path=README.md", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitStatus, req)
	assertOK(t, w)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["isGit"])
	assert.Equal(t, false, resp["hasUncommitted"])
}

func TestServeGitStatus_NotGitRepo(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/git/status?path=README.md", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitStatus, req)
	assertOK(t, w)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["isGit"])
	assert.Equal(t, false, resp["hasUncommitted"])
}

func TestServeGitStatus_MissingPath(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)

	req := newRequest(t, http.MethodGet, "/api/git/status", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitStatus, req)
	assertStatus(t, w, http.StatusBadRequest)
}

// --- ServeGitHistory ---

func TestServeGitHistory_WithCommits(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)

	// Make another commit on the file
	createTestFile(t, env.ProjectDir, "README.md", "# Updated")
	gitCommitAll(t, env.ProjectDir, "update readme")

	req := newRequest(t, http.MethodGet, "/api/git/history?path=README.md", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitHistory, req)
	assertOK(t, w)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	commits, ok := resp["commits"].([]interface{})
	assert.True(t, ok)
	assert.GreaterOrEqual(t, len(commits), 2)
}

func TestServeGitHistory_NotGitRepo(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/git/history?path=README.md", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitHistory, req)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestServeGitHistory_MissingPath(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)

	req := newRequest(t, http.MethodGet, "/api/git/history", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitHistory, req)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestServeGitHistory_PathTraversal(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)

	req := newRequest(t, http.MethodGet, "/api/git/history?path=../../../etc/passwd", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitHistory, req)
	assertStatus(t, w, http.StatusForbidden)
}

// --- ServeGitDiff ---

func TestServeGitDiff_UncommittedChanges(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)

	// Modify file without committing
	createTestFile(t, env.ProjectDir, "README.md", "# Modified content")

	req := newRequest(t, http.MethodGet, "/api/git/diff?path=README.md", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitDiff, req)
	assertOK(t, w)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["empty"])
	assert.Contains(t, resp["diff"], "# Modified content")
}

func TestServeGitDiff_NoChanges(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)

	req := newRequest(t, http.MethodGet, "/api/git/diff?path=README.md", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitDiff, req)
	assertOK(t, w)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["empty"])
}

func TestServeGitDiff_NotGitRepo(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/git/diff?path=README.md", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitDiff, req)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestServeGitDiff_SpecificCommit(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)

	// Make a second commit
	createTestFile(t, env.ProjectDir, "README.md", "# Updated")
	gitCommitAll(t, env.ProjectDir, "update readme")
	sha := getHeadSHA(t, env.ProjectDir)

	req := newRequest(t, http.MethodGet, "/api/git/diff?path=README.md&commit="+sha, nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitDiff, req)
	assertOK(t, w)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["empty"])
	assert.Contains(t, resp["diff"], "# Updated")
}

// --- ServeGitProjectHistory ---

func TestServeGitProjectHistory_WithCommits(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)

	req := newRequest(t, http.MethodGet, "/api/git/project-history", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitProjectHistory, req)
	assertOK(t, w)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["isGit"])
	commits, ok := resp["commits"].([]interface{})
	assert.True(t, ok)
	assert.GreaterOrEqual(t, len(commits), 1)
}

func TestServeGitProjectHistory_NotGitRepo(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/git/project-history", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitProjectHistory, req)
	assertOK(t, w)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["isGit"])
	commits, ok := resp["commits"].([]interface{})
	assert.True(t, ok)
	assert.Equal(t, 0, len(commits))
	assert.Equal(t, false, resp["hasMore"])
}

func TestServeGitProjectHistory_Pagination(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)

	// Create multiple commits
	for i := 0; i < 5; i++ {
		createTestFile(t, env.ProjectDir, fmt.Sprintf("file%d.txt", i), "content")
		gitCommitAll(t, env.ProjectDir, fmt.Sprintf("add file%d", i))
	}

	// Without skip
	req := newRequest(t, http.MethodGet, "/api/git/project-history", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitProjectHistory, req)
	assertOK(t, w)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	commits, _ := resp["commits"].([]interface{})
	totalCommits := len(commits)
	assert.GreaterOrEqual(t, totalCommits, 6) // initial + 5

	// With skip=1
	req2 := newRequest(t, http.MethodGet, "/api/git/project-history?skip=1", nil)
	withProjectCookie(req2, env.ProjectDir)

	w2 := callHandler(ServeGitProjectHistory, req2)
	assertOK(t, w2)

	var resp2 map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &resp2)
	commits2, _ := resp2["commits"].([]interface{})
	assert.Equal(t, totalCommits-1, len(commits2))
}

// --- ServeGitCommitFiles ---

func TestServeGitCommitFiles_Success(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)

	// Make a commit that adds a new file
	createTestFile(t, env.ProjectDir, "newfile.txt", "hello")
	gitCommitAll(t, env.ProjectDir, "add newfile")
	sha := getHeadSHA(t, env.ProjectDir)

	req := newRequest(t, http.MethodGet, "/api/git/commit-files?sha="+sha, nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitCommitFiles, req)
	assertOK(t, w)

	var files []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &files)
	assert.GreaterOrEqual(t, len(files), 1)

	// Check that newfile.txt is in the list
	found := false
	for _, f := range files {
		if f["path"] == "newfile.txt" {
			found = true
			assert.Equal(t, "A", f["type"])
			break
		}
	}
	assert.True(t, found, "expected newfile.txt in commit files")
}

func TestServeGitCommitFiles_MissingSHA(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)

	req := newRequest(t, http.MethodGet, "/api/git/commit-files", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitCommitFiles, req)
	assertStatus(t, w, http.StatusBadRequest)
}

// --- ServeGitFileDiff ---

func TestServeGitFileDiff_SpecificCommit(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)

	// Make a second commit modifying README
	createTestFile(t, env.ProjectDir, "README.md", "# Changed")
	gitCommitAll(t, env.ProjectDir, "change readme")
	sha := getHeadSHA(t, env.ProjectDir)

	req := newRequest(t, http.MethodGet, "/api/git/file-diff?sha="+sha+"&path=README.md", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitFileDiff, req)
	assertOK(t, w)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["empty"])
	assert.Contains(t, resp["diff"], "# Changed")
}

func TestServeGitFileDiff_HeadDiff(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)

	// Modify without committing
	createTestFile(t, env.ProjectDir, "README.md", "# Working tree change")

	req := newRequest(t, http.MethodGet, "/api/git/file-diff?sha=HEAD&path=README.md", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitFileDiff, req)
	assertOK(t, w)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["empty"])
	assert.Contains(t, resp["diff"], "# Working tree change")
}

func TestServeGitFileDiff_MissingSHA(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)

	req := newRequest(t, http.MethodGet, "/api/git/file-diff?path=README.md", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitFileDiff, req)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestServeGitFileDiff_MissingPath(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)
	sha := getHeadSHA(t, env.ProjectDir)

	req := newRequest(t, http.MethodGet, "/api/git/file-diff?sha="+sha, nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitFileDiff, req)
	assertStatus(t, w, http.StatusBadRequest)
}

// --- ServeGitWorkingTreeFiles ---

func TestServeGitWorkingTreeFiles_UncommittedFiles(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)

	// Modify tracked file and add a new file
	createTestFile(t, env.ProjectDir, "README.md", "# Changed")
	createTestFile(t, env.ProjectDir, "new.txt", "new file")

	req := newRequest(t, http.MethodGet, "/api/git/working-tree-files", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitWorkingTreeFiles, req)
	assertOK(t, w)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["isGit"])
	assert.Equal(t, true, resp["hasUncommitted"])
}

func TestServeGitWorkingTreeFiles_NotGitRepo(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	req := newRequest(t, http.MethodGet, "/api/git/working-tree-files", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitWorkingTreeFiles, req)
	assertOK(t, w)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["isGit"])
	assert.Equal(t, false, resp["hasUncommitted"])
}

func TestServeGitWorkingTreeFiles_SpecificFileCheck(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)

	// Modify a tracked file
	createTestFile(t, env.ProjectDir, "README.md", "# Changed")

	req := newRequest(t, http.MethodGet, "/api/git/working-tree-files?path=README.md", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitWorkingTreeFiles, req)
	assertOK(t, w)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["isGit"])
	assert.Equal(t, true, resp["hasUncommitted"])
}

func TestServeGitWorkingTreeFiles_NoChanges(t *testing.T) {
	env, teardown := setupTestEnv(t)
	defer teardown()

	initGitRepo(t, env.ProjectDir)

	req := newRequest(t, http.MethodGet, "/api/git/working-tree-files", nil)
	withProjectCookie(req, env.ProjectDir)

	w := callHandler(ServeGitWorkingTreeFiles, req)
	assertOK(t, w)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["isGit"])
	assert.Equal(t, false, resp["hasUncommitted"])
}

// --- validateFilePath ---

func TestValidateFilePath_ValidRelative(t *testing.T) {
	dir := t.TempDir()
	absPath, ok := model.ValidatePath(dir, "subdir/file.txt")
	assert.True(t, ok)
	expected := filepath.Join(dir, "subdir", "file.txt")
	assert.Equal(t, expected, absPath)
}

func TestValidateFilePath_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	absPath, ok := model.ValidatePath(dir, "../../../etc/passwd")
	assert.False(t, ok)
	// absPath may be non-empty (the resolved path), but ok must be false
	_ = absPath
}

func TestValidateFilePath_SimpleFile(t *testing.T) {
	dir := t.TempDir()
	absPath, ok := model.ValidatePath(dir, "README.md")
	assert.True(t, ok)
	assert.Equal(t, filepath.Join(dir, "README.md"), absPath)
}
