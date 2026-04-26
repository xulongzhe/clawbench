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
	SHA     string   `json:"sha"`
	Parents []string `json:"parents"` // parent commit SHAs (empty for initial commit, 2+ for merge)
	Msg     string   `json:"msg"`
	Date    string   `json:"date"`
	Author  string   `json:"author"`
	Refs    []string `json:"refs"` // branch/tag names decorating this commit
}

// parseGitLog parses git log output (format: %H|%P|%s|%ad|%an%d) into commitInfo slice.
// %P gives space-separated parent SHAs, %d gives decoration refs like " (HEAD -> main)".
func parseGitLog(output string) []commitInfo {
	var commits []commitInfo
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		// Split into 5 parts: SHA, parents, subject, date, author+refs
		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 5 {
			// Need all 5 parts: SHA, parents, subject, date, author+refs
			continue
		}

		// Parse parents (space-separated, may be empty for root commit)
		var parents []string
		if p := strings.TrimSpace(parts[1]); p != "" {
			parents = strings.Fields(p)
		}

		// The last part contains "author<optional refs>"
		// refs from %d appear as " (HEAD -> main, tag: v1.0)" appended after author
		authorAndRefs := parts[4]
		var author string
		var refs []string
		if idx := strings.LastIndex(authorAndRefs, " ("); idx >= 0 {
			author = strings.TrimSpace(authorAndRefs[:idx])
			refs = parseDecorateRefs(authorAndRefs[idx:])
		} else {
			author = strings.TrimSpace(authorAndRefs)
		}

		commits = append(commits, commitInfo{
			SHA:     parts[0],
			Parents: parents,
			Msg:     parts[2],
			Date:    parts[3],
			Author:  author,
			Refs:    refs,
		})
	}
	return commits
}

// parseDecorateRefs parses git decoration string like " (HEAD -> main, origin/main, tag: v1.0)"
// into a clean list of ref names.
func parseDecorateRefs(s string) []string {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '(' || s[len(s)-1] != ')' {
		return nil
	}
	s = s[1 : len(s)-1] // strip parens

	var refs []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Skip "HEAD -> " prefix, keep the branch name it points to
		if strings.HasPrefix(part, "HEAD -> ") {
			refs = append(refs, "HEAD")
			part = strings.TrimPrefix(part, "HEAD -> ")
		}
		// Remote-tracking refs are already excluded by --decorate-refs-exclude,
		// so no need to filter by "/" here. This preserves local branches
		// with slashes like "feature/login".
		if strings.HasPrefix(part, "tag: ") {
			refs = append(refs, part) // keep "tag: v1.0" format
		} else if part != "" {
			refs = append(refs, part)
		}
	}
	return refs
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
	// Format: SHA|parents|subject|date|author+refs
	// --topo-order ensures branches display contiguously
	// --decorate-refs-exclude hides remote-tracking refs
	logArgs := []string{"log", "--format=%H|%P|%s|%ad|%an%d", "--date=iso", "--topo-order", "--decorate-refs-exclude=refs/remotes", "-30"}
	if skip > 0 {
		logArgs = append(logArgs, "--skip", fmt.Sprintf("%d", skip))
	}
	cmd := exec.Command("git", logArgs...)
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
// For working tree diffs, it combines both staged and unstaged changes so that
// users see the complete picture of uncommitted modifications.
func gitDiff(projectPath, relPath, commit string) ([]byte, error) {
	if commit == "" || commit == "HEAD" {
		// Staged changes (diff between HEAD and index)
		cmdCached := exec.Command("git", "diff", "--cached", "--", relPath)
		cmdCached.Dir = projectPath
		cached, cachedErr := cmdCached.CombinedOutput()

		// Unstaged changes (diff between index and working tree)
		cmdUnstaged := exec.Command("git", "diff", "--", relPath)
		cmdUnstaged.Dir = projectPath
		unstaged, unstagedErr := cmdUnstaged.CombinedOutput()

		var combined []byte
		if cachedErr == nil && len(cached) > 0 {
			combined = append(combined, cached...)
		}
		if unstagedErr == nil && len(unstaged) > 0 {
			if len(combined) > 0 {
				combined = append(combined, '\n')
			}
			combined = append(combined, unstaged...)
		}

		if cachedErr != nil && unstagedErr != nil {
			return nil, cachedErr
		}
		return combined, nil
	}

	cmd := exec.Command("git", "show", commit, "--", relPath)
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

	// git log --format="%H|%P|%s|%ad|%an%d" --date=iso --topo-order -- <path>
	cmd := exec.Command("git", "log", "--format=%H|%P|%s|%ad|%an%d", "--date=iso", "--topo-order", "--decorate-refs-exclude=refs/remotes", "--", relPath)
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

// wtFileInfo extends commitInfo with a staged flag for working tree files.
type wtFileInfo struct {
	Path   string `json:"path"`
	Type   string `json:"type"`   // A=added, M=modified, D=deleted, ?=untracked
	Staged bool   `json:"staged"` // true if changes are staged (in index)
}

// parseGitStatusPorcelain parses `git status --porcelain` output into wtFileInfo slice.
// XY format: X=index status, Y=worktree status. ?=untracked.
// Path extraction: skip the 2-char XY prefix, then skip any leading spaces/tabs.
func parseGitStatusPorcelain(output string) []wtFileInfo {
	var files []wtFileInfo
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if len(line) < 3 {
			continue
		}
		x := line[0] // index (staged) status
		y := line[1] // worktree (unstaged) status
		// Skip the 2-char XY status, then skip leading whitespace to get path
		path := strings.TrimLeft(line[2:], " \t")
		// Handle quoted paths (git quotes paths with special chars)
		path = strings.Trim(path, "\"")

		// Determine display type and staged status
		switch {
		case x == '?' && y == '?':
			// Untracked file
			files = append(files, wtFileInfo{Path: path, Type: "?", Staged: false})
		case x != ' ' && x != '?':
			// Staged change (add/modify/delete/rename)
			t := string(x)
			if t == "R" {
				// Rename: path is "old -> new", extract new name after arrow
				if idx := strings.Index(path, "->"); idx >= 0 {
					path = strings.TrimLeft(path[idx+2:], " ")
				}
			}
			files = append(files, wtFileInfo{Path: path, Type: t, Staged: true})
			// If also modified in worktree, add a second entry
			if y == 'M' {
				files = append(files, wtFileInfo{Path: path, Type: "M", Staged: false})
			}
		case y != ' ':
			// Unstaged change only
			files = append(files, wtFileInfo{Path: path, Type: string(y), Staged: false})
		}
	}
	return files
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
		// Also check untracked
		if !hasUncommitted {
			lsCmd := exec.Command("git", "ls-files", "--error-unmatch", relPath)
			lsCmd.Dir = projectPath
			_, lsErr := lsCmd.CombinedOutput()
			hasUncommitted = lsErr != nil // not tracked = untracked file
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"isGit": true, "hasUncommitted": hasUncommitted, "files": []interface{}{}})
		return
	}

	// For project: return all uncommitted files using git status --porcelain
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = projectPath
	output, _ := cmd.CombinedOutput()
	allFiles := parseGitStatusPorcelain(string(output))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"isGit":          true,
		"hasUncommitted": len(allFiles) > 0,
		"files":          allFiles,
	})
}
