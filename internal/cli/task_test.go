package cli

import (
	"os"
	"path/filepath"
	"testing"

	"clawbench/internal/model"

	"github.com/stretchr/testify/assert"
)

func TestRunTaskCommand_NoArgs(t *testing.T) {
	// No args now prints help and returns 0
	exitCode := RunTaskCommand([]string{})
	assert.Equal(t, 0, exitCode)
}

func TestRunTaskCommand_HelpFlag(t *testing.T) {
	exitCode := RunTaskCommand([]string{"--help"})
	assert.Equal(t, 0, exitCode)
}

func TestRunTaskCommand_ShortHelpFlag(t *testing.T) {
	exitCode := RunTaskCommand([]string{"-h"})
	assert.Equal(t, 0, exitCode)
}

func TestRunTaskCommand_UnknownSubcommand(t *testing.T) {
	exitCode := RunTaskCommand([]string{"foo"})
	assert.Equal(t, 1, exitCode)
}

func TestCreateTask_MissingFields(t *testing.T) {
	exitCode := RunTaskCommand([]string{
		"create",
		"--name", "Test Task",
	})
	assert.Equal(t, 1, exitCode)
}

func TestCreateTask_ScheduledExecution(t *testing.T) {
	os.Setenv("CLAWBENCH_SCHEDULED", "1")
	defer os.Unsetenv("CLAWBENCH_SCHEDULED")

	exitCode := RunTaskCommand([]string{
		"create",
		"--name", "Test Task",
		"--cron", "0 9 * * *",
		"--agent", "codebuddy",
		"--prompt", "Test",
	})
	assert.Equal(t, 1, exitCode)
}

func TestCreateTask_LimitedRepeatWithoutMaxRuns(t *testing.T) {
	exitCode := RunTaskCommand([]string{
		"create",
		"--name", "Test Task",
		"--cron", "0 9 * * *",
		"--agent", "codebuddy",
		"--prompt", "Test",
		"--repeat", "limited",
	})
	assert.Equal(t, 1, exitCode)
}

func TestCreateTask_ServerNotReachable(t *testing.T) {
	tmpDir := t.TempDir()
	model.BinDir = tmpDir
	model.ConfigInstance = model.Config{
		WatchDir: tmpDir,
		Port:     59999,
	}

	exitCode := RunTaskCommand([]string{
		"create",
		"--name", "Test Task",
		"--cron", "0 9 * * *",
		"--agent", "codebuddy",
		"--prompt", "Test",
	})
	assert.Equal(t, 1, exitCode)
}

func TestDeleteTask_NoTaskID(t *testing.T) {
	exitCode := RunTaskCommand([]string{"delete"})
	assert.Equal(t, 1, exitCode)
}

func TestPauseTask_NoTaskID(t *testing.T) {
	exitCode := RunTaskCommand([]string{"pause"})
	assert.Equal(t, 1, exitCode)
}

func TestResumeTask_NoTaskID(t *testing.T) {
	exitCode := RunTaskCommand([]string{"resume"})
	assert.Equal(t, 1, exitCode)
}

func TestTriggerTask_NoTaskID(t *testing.T) {
	exitCode := RunTaskCommand([]string{"trigger"})
	assert.Equal(t, 1, exitCode)
}

func TestUpdateTask_NoTaskID(t *testing.T) {
	exitCode := RunTaskCommand([]string{"update"})
	assert.Equal(t, 1, exitCode)
}

func TestUpdateTask_InvalidRepeat(t *testing.T) {
	exitCode := RunTaskCommand([]string{
		"update", "some-id",
		"--repeat", "invalid",
	})
	assert.Equal(t, 1, exitCode)
}

func TestCreateTask_InvalidRepeat(t *testing.T) {
	exitCode := RunTaskCommand([]string{
		"create",
		"--name", "Test",
		"--cron", "0 9 * * *",
		"--agent", "codebuddy",
		"--prompt", "Test",
		"--repeat", "invalid",
	})
	assert.Equal(t, 1, exitCode)
}

func TestReorderFlagsFirst_AllFlagsFirst(t *testing.T) {
	// Flags already before positional — no change needed
	args := []string{"--project", "/path", "--prompt", "hello", "task-abc"}
	result := reorderFlagsFirst(args)
	assert.Equal(t, []string{"--project", "/path", "--prompt", "hello", "task-abc"}, result)
}

func TestReorderFlagsFirst_PositionalBetweenFlags(t *testing.T) {
	// The exact bug: task-ID before --prompt causes Go flag to skip --prompt
	args := []string{"task-abc", "--prompt", "hello", "--project", "/path"}
	result := reorderFlagsFirst(args)
	assert.Equal(t, []string{"--prompt", "hello", "--project", "/path", "task-abc"}, result)
}

func TestReorderFlagsFirst_MixedOrder(t *testing.T) {
	args := []string{"--project", "/path", "task-abc", "--prompt", "hello"}
	result := reorderFlagsFirst(args)
	assert.Equal(t, []string{"--project", "/path", "--prompt", "hello", "task-abc"}, result)
}

func TestReorderFlagsFirst_FlagWithEquals(t *testing.T) {
	args := []string{"task-abc", "--project=/path", "--prompt=hello"}
	result := reorderFlagsFirst(args)
	assert.Equal(t, []string{"--project=/path", "--prompt=hello", "task-abc"}, result)
}

func TestReorderFlagsFirst_NoFlags(t *testing.T) {
	args := []string{"task-abc"}
	result := reorderFlagsFirst(args)
	assert.Equal(t, []string{"task-abc"}, result)
}

func TestReorderFlagsFirst_NoPositional(t *testing.T) {
	args := []string{"--project", "/path"}
	result := reorderFlagsFirst(args)
	assert.Equal(t, []string{"--project", "/path"}, result)
}

func TestReadFlagOrFile_PlainValue(t *testing.T) {
	val, err := readFlagOrFile("hello world")
	assert.NoError(t, err)
	assert.Equal(t, "hello world", val)
}

func TestReadFlagOrFile_FileReference(t *testing.T) {
	// Create a temp file with content
	tmpDir := t.TempDir()
	promptFile := filepath.Join(tmpDir, "prompt.txt")
	content := "This is a test prompt with $VARIABLE"
	err := os.WriteFile(promptFile, []byte(content), 0644)
	assert.NoError(t, err)

	val, err := readFlagOrFile("@" + promptFile)
	assert.NoError(t, err)
	assert.Equal(t, content, val)
}

func TestReadFlagOrFile_FileNotFound(t *testing.T) {
	_, err := readFlagOrFile("@/nonexistent/path/file.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read file")
}

func TestReadFlagOrFile_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	promptFile := filepath.Join(tmpDir, "empty.txt")
	err := os.WriteFile(promptFile, []byte(""), 0644)
	assert.NoError(t, err)

	val, err := readFlagOrFile("@" + promptFile)
	assert.NoError(t, err)
	assert.Equal(t, "", val)
}

func TestReadFlagOrFile_AtSignAlone(t *testing.T) {
	// "@" alone means read from a file named "" — should error
	_, err := readFlagOrFile("@")
	assert.Error(t, err)
}
