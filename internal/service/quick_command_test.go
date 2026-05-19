package service_test

import (
	"testing"

	"clawbench/internal/service"

	"github.com/stretchr/testify/assert"
)

// ---------- GetQuickCommands ----------

func TestGetQuickCommands_Empty(t *testing.T) {
	setupDB(t)

	cmds, err := service.GetQuickCommands()
	assert.NoError(t, err)
	assert.Nil(t, cmds)
}

// ---------- AddQuickCommand ----------

func TestAddQuickCommand_Single(t *testing.T) {
	setupDB(t)

	id, err := service.AddQuickCommand("List Files", "ls -la", false, false)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), id)

	cmds, err := service.GetQuickCommands()
	assert.NoError(t, err)
	assert.Len(t, cmds, 1)
	assert.Equal(t, int64(1), cmds[0].ID)
	assert.Equal(t, "List Files", cmds[0].Label)
	assert.Equal(t, "ls -la", cmds[0].Command)
}

func TestAddQuickCommand_Multiple(t *testing.T) {
	setupDB(t)

	id1, err := service.AddQuickCommand("cmd1", "ls", false, false)
	assert.NoError(t, err)
	id2, err := service.AddQuickCommand("cmd2", "pwd", false, false)
	assert.NoError(t, err)

	assert.True(t, id2 > id1, "IDs should auto-increment")

	cmds, _ := service.GetQuickCommands()
	assert.Len(t, cmds, 2)
}

func TestAddQuickCommand_AutoExecuteClearsPrevious(t *testing.T) {
	setupDB(t)

	// Add first auto-execute command
	_, err := service.AddQuickCommand("Auto1", "cmd1", false, true)
	assert.NoError(t, err)

	// Add second auto-execute command — should clear the first
	_, err = service.AddQuickCommand("Auto2", "cmd2", false, true)
	assert.NoError(t, err)

	cmds, _ := service.GetQuickCommands()
	assert.Len(t, cmds, 2)
	// First command should no longer be auto_execute
	assert.False(t, cmds[0].AutoExecute)
	assert.True(t, cmds[1].AutoExecute)
}

// ---------- UpdateQuickCommand ----------

func TestUpdateQuickCommand(t *testing.T) {
	setupDB(t)

	service.AddQuickCommand("Old Label", "old cmd", false, false)

	err := service.UpdateQuickCommand(1, "New Label", "new cmd", false, false)
	assert.NoError(t, err)

	cmds, _ := service.GetQuickCommands()
	assert.Len(t, cmds, 1)
	assert.Equal(t, "New Label", cmds[0].Label)
	assert.Equal(t, "new cmd", cmds[0].Command)
}

func TestUpdateQuickCommand_Nonexistent(t *testing.T) {
	setupDB(t)

	err := service.UpdateQuickCommand(999, "x", "y", false, false)
	assert.NoError(t, err) // UPDATE on non-existent row is a no-op
}

// ---------- DeleteQuickCommand ----------

func TestDeleteQuickCommand(t *testing.T) {
	setupDB(t)

	service.AddQuickCommand("cmd1", "ls", false, false)
	service.AddQuickCommand("cmd2", "pwd", false, false)

	err := service.DeleteQuickCommand(1)
	assert.NoError(t, err)

	cmds, _ := service.GetQuickCommands()
	assert.Len(t, cmds, 1)
	assert.Equal(t, "pwd", cmds[0].Command)
}

func TestDeleteQuickCommand_Nonexistent(t *testing.T) {
	setupDB(t)

	err := service.DeleteQuickCommand(999)
	assert.NoError(t, err) // DELETE on non-existent row is a no-op
}

// ---------- ReorderQuickCommands ----------

func TestReorderQuickCommands(t *testing.T) {
	setupDB(t)

	service.AddQuickCommand("A", "a", false, false)
	service.AddQuickCommand("B", "b", false, false)
	service.AddQuickCommand("C", "c", false, false)

	// Reverse order: C, B, A
	err := service.ReorderQuickCommands([]int64{3, 2, 1})
	assert.NoError(t, err)

	cmds, _ := service.GetQuickCommands()
	assert.Len(t, cmds, 3)
	assert.Equal(t, "C", cmds[0].Label)
	assert.Equal(t, "A", cmds[2].Label)
}

func TestReorderQuickCommands_EmptyIDs(t *testing.T) {
	setupDB(t)

	service.AddQuickCommand("A", "a", false, false)

	err := service.ReorderQuickCommands([]int64{})
	assert.NoError(t, err)

	cmds, _ := service.GetQuickCommands()
	assert.Len(t, cmds, 1)
}
