package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"clawbench/internal/model"
)

// commitInfo represents a git commit in API responses.
type commitInfo struct {
	SHA    string `json:"sha"`
	Msg    string `json:"msg"`
	Date   string `json:"date"`
	Author string `json:"author"`
}

// parseGitLog parses git log output (format: %H|%s|%ad|%an) into commitInfo slice.
func parseGitLog(output string) []commitInfo {
	var commits []commitInfo
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) >= 3 {
			commits = append(commits, commitInfo{
				SHA:    parts[0],
				Msg:    parts[1],
				Date:   parts[2],
				Author: parts[3],
			})
		}
	}
	return commits
}

// isGitRepo checks if the given path is inside a git repository.
func isGitRepo(projectPath string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = projectPath
	_, err := cmd.Output()
	return err == nil
}

// ServeGitProjectHistory returns commit history for the entire project.
// Supports pagination via ?skip=N (skips N commits).
func ServeGitProjectHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	if !isGitRepo(projectPath) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"isGit":   false,
			"commits": []interface{}{},
			"hasMore": false,
		})
		return
	}

	skip := 0
	if s := r.URL.Query().Get("skip"); s != "" {
		fmt.Sscanf(s, "%d", &skip)
	}

	// git log for entire project, with optional skip
	cmd := exec.Command("git", "log", "--format=%H|%s|%ad|%an", "--date=iso", "-30")
	if skip > 0 {
		cmd = exec.Command("git", "log", "--format=%H|%s|%ad|%an", "--date=iso", "--skip", fmt.Sprintf("%d", skip), "-30")
	}
	cmd.Dir = projectPath
	output, _ := cmd.CombinedOutput()

	commits := parseGitLog(string(output))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"isGit":   true,
		"commits": commits,
		"hasMore": len(commits) == 30,
	})
}

// ServeGitInit initializes a new git repository in the project directory.
func ServeGitInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	if isGitRepo(projectPath) {
		model.WriteErrorf(w, http.StatusBadRequest, "already a git repository")
		return
	}

	// git init
	cmd := exec.Command("git", "init")
	cmd.Dir = projectPath
	_, err := cmd.CombinedOutput()
	if err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("failed to init git repository")))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// gitDiff returns the diff for a file at a specific commit (or HEAD for working tree).
func gitDiff(projectPath, relPath, commit string) ([]byte, error) {
	var cmd *exec.Cmd
	if commit == "" || commit == "HEAD" {
		cmd = exec.Command("git", "diff", "HEAD", "--", relPath)
	} else {
		cmd = exec.Command("git", "show", commit, "--", relPath)
	}
	cmd.Dir = projectPath
	return cmd.CombinedOutput()
}

// writeDiffResponse writes the diff response as JSON.
func writeDiffResponse(w http.ResponseWriter, output []byte, cmdErr error) {
	if cmdErr != nil && len(output) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"diff": "", "empty": true})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"diff":  string(output),
		"empty": len(strings.TrimSpace(string(output))) == 0,
	})
}

// ServeGitFileDiff returns the diff for a specific file in a specific commit
// (comparing the commit vs its parent), or working tree diff if sha is "HEAD".
func ServeGitFileDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}
	if !isGitRepo(projectPath) {
		model.WriteErrorf(w, http.StatusBadRequest, "not a git repository")
		return
	}

	sha := r.URL.Query().Get("sha")
	filePath := r.URL.Query().Get("path")
	if sha == "" || filePath == "" {
		model.WriteErrorf(w, http.StatusBadRequest, "missing sha or path")
		return
	}

	if _, ok := model.ValidatePath(projectPath, filePath); !ok {
		model.WriteError(w, model.Forbidden(nil, "access denied"))
		return
	}

	output, err := gitDiff(projectPath, filePath, sha)
	writeDiffResponse(w, output, err)
}

// ServeGitCommitFiles returns the list of files modified in a specific commit.
func ServeGitCommitFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}
	if !isGitRepo(projectPath) {
		model.WriteErrorf(w, http.StatusBadRequest, "not a git repository")
		return
	}

	sha := r.URL.Query().Get("sha")
	if sha == "" {
		model.WriteErrorf(w, http.StatusBadRequest, "missing sha")
		return
	}

	// git diff-tree --no-commit-id --name-status -r <sha>
	cmd := exec.Command("git", "diff-tree", "--no-commit-id", "--name-status", "-r", sha)
	cmd.Dir = projectPath
	output, err := cmd.CombinedOutput()

	type fileInfo struct {
		Path string `json:"path"`
		Type string `json:"type"` // A=added, M=modified, D=deleted
	}

	var files []fileInfo
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "\t", 2)
			if len(parts) == 2 {
				files = append(files, fileInfo{
					Type: parts[0],
					Path: parts[1],
				})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

// ServeGitHistory returns commit history for a specific file.
func ServeGitHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}
	if !isGitRepo(projectPath) {
		model.WriteErrorf(w, http.StatusBadRequest, "not a git repository")
		return
	}

	relPath := r.URL.Query().Get("path")
	if relPath == "" {
		model.WriteErrorf(w, http.StatusBadRequest, "missing path")
		return
	}

	if _, ok := model.ValidatePath(projectPath, relPath); !ok {
		model.WriteError(w, model.Forbidden(nil, "access denied"))
		return
	}

	// Check if the file is tracked by Git (in the index or any commit).
	// git ls-files --error-unmatch exits with code 0 and produces no output
	// only when the file is tracked. Otherwise it exits with code 128.
	lsCmd := exec.Command("git", "ls-files", "--error-unmatch", relPath)
	lsCmd.Dir = projectPath
	lsOut, lsErr := lsCmd.CombinedOutput()
	untracked := lsErr != nil || len(lsOut) == 0

	// git log --format="%H|%s|%ad|%an" --date=iso -- <path>
	cmd := exec.Command("git", "log", "--format=%H|%s|%ad|%an",
		"--date=iso", "--", relPath)
	cmd.Dir = projectPath
	output, _ := cmd.CombinedOutput()

	commits := parseGitLog(string(output))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"commits": commits, "untracked": untracked})
}

// ServeGitDiff returns the diff for a specific commit or the working tree diff.
func ServeGitDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}
	if !isGitRepo(projectPath) {
		model.WriteErrorf(w, http.StatusBadRequest, "not a git repository")
		return
	}

	relPath := r.URL.Query().Get("path")
	if relPath == "" {
		model.WriteErrorf(w, http.StatusBadRequest, "missing path")
		return
	}

	if _, ok := model.ValidatePath(projectPath, relPath); !ok {
		model.WriteError(w, model.Forbidden(nil, "access denied"))
		return
	}

	commit := r.URL.Query().Get("commit")
	output, err := gitDiff(projectPath, relPath, commit)
	writeDiffResponse(w, output, err)
}

// ServeGitStatus returns whether there are uncommitted changes for the file.
func ServeGitStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}
	if !isGitRepo(projectPath) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"isGit": false, "hasUncommitted": false})
		return
	}

	relPath := r.URL.Query().Get("path")
	if relPath == "" {
		model.WriteErrorf(w, http.StatusBadRequest, "missing path")
		return
	}

	if _, ok := model.ValidatePath(projectPath, relPath); !ok {
		model.WriteError(w, model.Forbidden(nil, "access denied"))
		return
	}

	cmd := exec.Command("git", "diff", "--stat", "HEAD", "--", relPath)
	cmd.Dir = projectPath
	output, err := cmd.CombinedOutput()

	w.Header().Set("Content-Type", "application/json")
	hasUncommitted := err == nil && len(strings.TrimSpace(string(output))) > 0
	json.NewEncoder(w).Encode(map[string]interface{}{"isGit": true, "hasUncommitted": hasUncommitted})
}

// ServeGitWorkingTreeFiles returns uncommitted file changes for the project or a specific file.
func ServeGitWorkingTreeFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}
	if !isGitRepo(projectPath) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"isGit": false, "hasUncommitted": false, "files": []interface{}{}})
		return
	}

	relPath := r.URL.Query().Get("path")

	// For specific file: check if it has uncommitted changes
	if relPath != "" {
		if _, ok := model.ValidatePath(projectPath, relPath); !ok {
			model.WriteError(w, model.Forbidden(nil, "access denied"))
			return
		}
		cmd := exec.Command("git", "diff", "--name-status", "HEAD", "--", relPath)
		cmd.Dir = projectPath
		output, err := cmd.CombinedOutput()
		hasUncommitted := err == nil && len(strings.TrimSpace(string(output))) > 0
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"isGit": true, "hasUncommitted": hasUncommitted, "files": []interface{}{}})
		return
	}

	// For project: return all uncommitted files (staged + unstaged)
	var allFiles []FileInfo

	// Staged changes
	cmdStaged := exec.Command("git", "diff", "--cached", "--name-status")
	cmdStaged.Dir = projectPath
	outputStaged, _ := cmdStaged.CombinedOutput()
	for _, line := range strings.Split(strings.TrimSpace(string(outputStaged)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 {
			// Staged files marked with type prefix "S"
			t := "S" + parts[0]
			allFiles = append(allFiles, FileInfo{Path: parts[1], Type: t})
		}
	}

	// Unstaged changes
	cmdUnstaged := exec.Command("git", "diff", "--name-status", "HEAD")
	cmdUnstaged.Dir = projectPath
	outputUnstaged, _ := cmdUnstaged.CombinedOutput()
	for _, line := range strings.Split(strings.TrimSpace(string(outputUnstaged)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 {
			// Only add if not already staged
			already := false
			for _, f := range allFiles {
				if f.Path == parts[1] {
					already = true
					break
				}
			}
			if !already {
				allFiles = append(allFiles, FileInfo{Path: parts[1], Type: parts[0]})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"isGit":          true,
		"hasUncommitted": len(allFiles) > 0,
		"files":          allFiles,
	})
}
