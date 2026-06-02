package handler

import (
	"strings"
	"testing"

	"clawbench/internal/model"

	"github.com/stretchr/testify/assert"
)

// --- processAtCommand tests ---

func TestProcessAtCommand_ChatSearchInjects(t *testing.T) {
	model.ClawbenchBin = "/usr/local/bin/clawbench"
	defer func() { model.ClawbenchBin = "" }()

	result := processAtCommand("@chatsearch fix login bug", "/project", "session-123")

	// Must contain injection template
	assert.Contains(t, result, "historical conversation search")
	assert.Contains(t, result, "/usr/local/bin/clawbench rag search")
	assert.Contains(t, result, "--project /project")
	assert.Contains(t, result, "--exclude-session-id session-123")
	// Must contain original message
	assert.Contains(t, result, "@chatsearch fix login bug")
	// Template must be prepended before the original message
	templateEnd := strings.Index(result, "@chatsearch fix login bug")
	assert.True(t, templateEnd > 0, "template should be prepended before original message")
}

func TestProcessAtCommand_TaskInjects(t *testing.T) {
	model.ClawbenchBin = "/usr/local/bin/clawbench"
	defer func() { model.ClawbenchBin = "" }()

	result := processAtCommand("@task daily build", "/project", "session-456")

	assert.Contains(t, result, "scheduled task management")
	assert.Contains(t, result, "/usr/local/bin/clawbench task")
	assert.Contains(t, result, "--project /project")
	assert.Contains(t, result, "@task daily build")
	templateEnd := strings.Index(result, "@task daily build")
	assert.True(t, templateEnd > 0, "template should be prepended before original message")
}

func TestProcessAtCommand_NoPrefixPassesThrough(t *testing.T) {
	result := processAtCommand("hello world", "/project", "session-123")
	assert.Equal(t, "hello world", result)
}

func TestProcessAtCommand_EmptyQueryReturnsRaw(t *testing.T) {
	// @chatsearch with only whitespace after should return the raw message
	// (caller handles the error response)
	result := processAtCommand("@chatsearch  ", "/project", "session-123")
	assert.Equal(t, "@chatsearch  ", result)
}

func TestProcessAtCommand_TaskEmptyDescReturnsInjected(t *testing.T) {
	model.ClawbenchBin = "/usr/local/bin/clawbench"
	defer func() { model.ClawbenchBin = "" }()

	// @task with just a space still injects — task description can be short
	result := processAtCommand("@task ", "/project", "session-123")
	assert.Contains(t, result, "scheduled task management")
}

func TestProcessAtCommand_PartialPrefixNoMatch(t *testing.T) {
	// @chat without "search" should not match
	result := processAtCommand("@chat something", "/project", "session-123")
	assert.Equal(t, "@chat something", result)
}

func TestProcessAtCommand_ChatSearchPlaceholderReplacement(t *testing.T) {
	model.ClawbenchBin = "/opt/clawbench/bin/clawbench"
	defer func() { model.ClawbenchBin = "" }()

	result := processAtCommand("@chatsearch auth bug", "/my/project", "sess-abc")

	assert.Contains(t, result, "/opt/clawbench/bin/clawbench rag search")
	assert.Contains(t, result, "--project /my/project")
	assert.Contains(t, result, "--exclude-session-id sess-abc")
	// No unreplaced placeholders
	assert.NotContains(t, result, "{{CLAWBENCH_BIN}}")
	assert.NotContains(t, result, "{{PROJECT_PATH}}")
	assert.NotContains(t, result, "{{SESSION_ID}}")
}

func TestProcessAtCommand_TaskPlaceholderReplacement(t *testing.T) {
	model.ClawbenchBin = "/opt/clawbench/bin/clawbench"
	defer func() { model.ClawbenchBin = "" }()

	result := processAtCommand("@task daily report", "/my/project", "sess-abc")

	assert.Contains(t, result, "/opt/clawbench/bin/clawbench task")
	assert.Contains(t, result, "--project /my/project")
	assert.NotContains(t, result, "{{CLAWBENCH_BIN}}")
	assert.NotContains(t, result, "{{PROJECT_PATH}}")
}

func TestProcessAtCommand_ChatSearchContainsXMLFormat(t *testing.T) {
	model.ClawbenchBin = "/usr/local/bin/clawbench"
	defer func() { model.ClawbenchBin = "" }()

	result := processAtCommand("@chatsearch test", "/project", "session-123")

	// Must instruct AI about XML output format
	assert.Contains(t, result, "<rag-results>")
	assert.Contains(t, result, "<rag-item>")
	assert.Contains(t, result, "<session-id>")
	assert.Contains(t, result, "<session-title>")
	assert.Contains(t, result, "<created-at>")
	assert.Contains(t, result, "<summary>")
}

func TestProcessAtCommand_TaskContainsScheduledTaskTag(t *testing.T) {
	model.ClawbenchBin = "/usr/local/bin/clawbench"
	defer func() { model.ClawbenchBin = "" }()

	result := processAtCommand("@task test task", "/project", "session-123")

	assert.Contains(t, result, "<scheduled-task")
	assert.Contains(t, result, "--agent-id")
}
