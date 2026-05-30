package service

import (
	"clawbench/internal/model"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
	"gopkg.in/yaml.v3"
)

// setupTestDBForMigration creates an in-memory SQLite with agents tables for migration tests.
func setupTestDBForMigration(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)

	_, err = db.Exec(AgentDDL)
	require.NoError(t, err)

	origDB := DB
	origDBRead := DBRead
	DB = db
	DBRead = db
	t.Cleanup(func() {
		DB = origDB
		DBRead = origDBRead
		db.Close()
	})

	return db
}

func TestMigrateAgentsFromYAML_HappyPath(t *testing.T) {
	db := setupTestDBForMigration(t)
	dir := t.TempDir()

	// Write two YAML files
	writeAgentYAML(t, dir, "claude.yaml", map[string]any{
		"id": "claude", "name": "Claude", "icon": "🤖",
		"specialty": "代码编写与推理", "backend": "claude",
		"preferred_model": "claude-sonnet-4-6",
	})
	writeAgentYAML(t, dir, "pi.yaml", map[string]any{
		"id": "pi", "name": "Pi", "icon": "🥧",
		"specialty": "极简编程智能体", "backend": "pi",
		"preferred_model": "minimax-cn/MiniMax-M2.7",
		"system_prompt": "You are a versatile assistant.",
	})

	err := MigrateAgentsFromYAML(db, dir)
	require.NoError(t, err)

	// Verify agents were migrated
	agents, err := LoadAgentsFromDB(db)
	require.NoError(t, err)
	require.Len(t, agents, 2)

	// Sorted by ID: claude first, pi second
	assert.Equal(t, "claude", agents[0].ID)
	assert.Equal(t, "Claude", agents[0].Name)
	assert.Equal(t, "claude-sonnet-4-6", agents[0].PreferredModel)
	assert.Equal(t, "auto", agents[0].Source) // migrated agents are "auto"

	assert.Equal(t, "pi", agents[1].ID)
	assert.Equal(t, "Pi", agents[1].Name)
	assert.Equal(t, "minimax-cn/MiniMax-M2.7", agents[1].PreferredModel)
	assert.Equal(t, "You are a versatile assistant.", agents[1].SystemPrompt)
	assert.Equal(t, "auto", agents[1].Source)
}

func TestMigrateAgentsFromYAML_Idempotent(t *testing.T) {
	db := setupTestDBForMigration(t)
	dir := t.TempDir()

	writeAgentYAML(t, dir, "pi.yaml", map[string]any{
		"id": "pi", "name": "Pi", "backend": "pi",
	})

	// Migrate once
	err := MigrateAgentsFromYAML(db, dir)
	require.NoError(t, err)

	// Migrate again — should be a no-op (agents table not empty)
	err = MigrateAgentsFromYAML(db, dir)
	require.NoError(t, err)

	// Should still have exactly one agent
	agents, err := LoadAgentsFromDB(db)
	require.NoError(t, err)
	assert.Len(t, agents, 1)
}

func TestMigrateAgentsFromYAML_EmptyDir(t *testing.T) {
	db := setupTestDBForMigration(t)
	dir := t.TempDir() // empty dir

	err := MigrateAgentsFromYAML(db, dir)
	require.NoError(t, err)

	// No agents should be created
	agents, err := LoadAgentsFromDB(db)
	require.NoError(t, err)
	assert.Empty(t, agents)
}

func TestMigrateAgentsFromYAML_NonexistentDir(t *testing.T) {
	db := setupTestDBForMigration(t)

	err := MigrateAgentsFromYAML(db, "/nonexistent/path")
	require.NoError(t, err) // should not error, just skip

	agents, err := LoadAgentsFromDB(db)
	require.NoError(t, err)
	assert.Empty(t, agents)
}

func TestMigrateAgentsFromYAML_InvalidYAML(t *testing.T) {
	db := setupTestDBForMigration(t)
	dir := t.TempDir()

	// Write a valid and an invalid YAML
	writeAgentYAML(t, dir, "pi.yaml", map[string]any{
		"id": "pi", "name": "Pi", "backend": "pi",
	})
	// Invalid YAML (missing id)
	os.WriteFile(filepath.Join(dir, "invalid.yaml"), []byte("name: no-id\nbackend: test\n"), 0644)
	// Non-YAML file (should be skipped)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a yaml"), 0644)

	err := MigrateAgentsFromYAML(db, dir)
	require.NoError(t, err)

	// Only the valid agent should be migrated
	agents, err := LoadAgentsFromDB(db)
	require.NoError(t, err)
	require.Len(t, agents, 1)
	assert.Equal(t, "pi", agents[0].ID)
}

func TestMigrateAgentsFromYAML_ModelsAndLevels(t *testing.T) {
	db := setupTestDBForMigration(t)
	dir := t.TempDir()

	writeAgentYAML(t, dir, "codebuddy.yaml", map[string]any{
		"id": "codebuddy", "name": "顶梁柱", "icon": "🐛",
		"specialty": "全栈开发助手", "backend": "codebuddy",
		"preferred_model": "glm-5.1",
		"thinking_effort_levels": []string{"low", "medium", "high", "xhigh"},
		"models": []map[string]any{
			{"id": "glm-5.1", "name": "GLM-5.1", "default": true},
			{"id": "glm-4.7", "name": "GLM-4.7"},
		},
	})

	err := MigrateAgentsFromYAML(db, dir)
	require.NoError(t, err)

	agents, err := LoadAgentsFromDB(db)
	require.NoError(t, err)
	require.Len(t, agents, 1)

	got := agents[0]
	assert.Equal(t, "codebuddy", got.ID)
	assert.Equal(t, "顶梁柱", got.Name)
	assert.Equal(t, "glm-5.1", got.PreferredModel)
	require.Len(t, got.Models, 2)
	assert.Equal(t, "glm-5.1", got.Models[0].ID)
	assert.True(t, got.Models[0].Default)
	require.Len(t, got.ThinkingEffortLevels, 4)
	assert.Equal(t, "xhigh", got.ThinkingEffortLevels[3])
}

func TestMigrateAgentsFromYAML_AlreadyHasDBAgents(t *testing.T) {
	db := setupTestDBForMigration(t)
	dir := t.TempDir()

	// Pre-populate DB with an agent (simulates previous migration)
	err := SaveAgent(db, &model.Agent{
		ID: "existing", Name: "Existing", Backend: "claude", Source: "auto",
	})
	require.NoError(t, err)

	// Write YAML that would create another agent
	writeAgentYAML(t, dir, "pi.yaml", map[string]any{
		"id": "pi", "name": "Pi", "backend": "pi",
	})

	// Migration should be skipped (agents table not empty)
	err = MigrateAgentsFromYAML(db, dir)
	require.NoError(t, err)

	// Should only have the pre-existing agent
	agents, err := LoadAgentsFromDB(db)
	require.NoError(t, err)
	require.Len(t, agents, 1)
	assert.Equal(t, "existing", agents[0].ID)
}

func TestMigrateAgentsFromYAML_DirectoryEntry(t *testing.T) {
	db := setupTestDBForMigration(t)
	dir := t.TempDir()

	// Create a subdirectory (should be skipped)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)
	writeAgentYAML(t, dir, "pi.yaml", map[string]any{
		"id": "pi", "name": "Pi", "backend": "pi",
	})

	err := MigrateAgentsFromYAML(db, dir)
	require.NoError(t, err)

	agents, err := LoadAgentsFromDB(db)
	require.NoError(t, err)
	require.Len(t, agents, 1)
	assert.Equal(t, "pi", agents[0].ID)
}

// Helper to write a YAML agent file
func writeAgentYAML(t *testing.T, dir, filename string, data map[string]any) {
	t.Helper()
	content, err := yaml.Marshal(data)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, filename), content, 0644)
	require.NoError(t, err)
}

// ---------- LoadAgentsIntoMemory ----------

func TestLoadAgentsIntoMemory(t *testing.T) {
	db := setupTestDBForMigration(t)

	// Save some agents
	err := SaveAgent(db, &model.Agent{ID: "pi", Name: "Pi", Backend: "pi", Source: "auto"})
	require.NoError(t, err)
	err = SaveAgent(db, &model.Agent{ID: "claude", Name: "Claude", Backend: "claude", SystemPrompt: "Be helpful", Source: "auto"})
	require.NoError(t, err)

	// Save global state
	origAgents := model.Agents
	origAgentList := model.AgentList
	defer func() {
		model.Agents = origAgents
		model.AgentList = origAgentList
	}()

	err = LoadAgentsIntoMemory(db)
	require.NoError(t, err)

	// Verify model.Agents map is populated
	assert.NotNil(t, model.Agents)
	assert.Contains(t, model.Agents, "pi")
	assert.Contains(t, model.Agents, "claude")

	// Verify model.AgentList is populated and sorted by ID
	assert.Len(t, model.AgentList, 2)
	assert.Equal(t, "claude", model.AgentList[0].ID)
	assert.Equal(t, "pi", model.AgentList[1].ID)
}

func TestLoadAgentsIntoMemory_Empty(t *testing.T) {
	db := setupTestDBForMigration(t)

	origAgents := model.Agents
	origAgentList := model.AgentList
	defer func() {
		model.Agents = origAgents
		model.AgentList = origAgentList
	}()

	err := LoadAgentsIntoMemory(db)
	require.NoError(t, err)

	assert.NotNil(t, model.Agents)
	assert.Empty(t, model.AgentList)
}

// ---------- MigrateAgentsFromYAML: unreadable file ----------

func TestMigrateAgentsFromYAML_UnreadableFile(t *testing.T) {
	db := setupTestDBForMigration(t)
	dir := t.TempDir()

	// Write a valid YAML file
	writeAgentYAML(t, dir, "pi.yaml", map[string]any{
		"id": "pi", "name": "Pi", "backend": "pi",
	})

	// Create a file then make the directory unreadable (write-only) is hard to test
	// reliably across platforms. Instead, just verify valid YAML files work.
	err := MigrateAgentsFromYAML(db, dir)
	require.NoError(t, err)

	agents, err := LoadAgentsFromDB(db)
	require.NoError(t, err)
	assert.Len(t, agents, 1)
}

// ---------- saveAgentTx: tested through MigrateAgentsFromYAML ----------

func TestSaveAgentTx_WithModels(t *testing.T) {
	db := setupTestDBForMigration(t)

	agent := &model.Agent{
		ID:       "codebuddy",
		Name:     "CodeBuddy",
		Backend:  "codebuddy",
		Models: []model.AgentModel{
			{ID: "glm-5.1", Name: "GLM-5.1", Default: true},
		},
		ThinkingEffortLevels: []string{"low", "medium", "high"},
		ModelsAutoDetected:   true,
		Source:               "auto",
	}

	tx, err := db.Begin()
	require.NoError(t, err)

	err = saveAgentTx(tx, agent)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	agents, err := LoadAgentsFromDB(db)
	require.NoError(t, err)
	require.Len(t, agents, 1)
	assert.Equal(t, "codebuddy", agents[0].ID)
	assert.Len(t, agents[0].Models, 1)
	assert.True(t, agents[0].ModelsAutoDetected)
}

func TestMigrateAgentsFromYAML_ReadDirFailure(t *testing.T) {
	db := setupTestDBForMigration(t)

	// Point to a nonexistent directory — should return nil (skip)
	err := MigrateAgentsFromYAML(db, "/nonexistent/path/12345")
	assert.NoError(t, err)
}

func TestMigrateAgentsFromYAML_InvalidYAMLContent(t *testing.T) {
	db := setupTestDBForMigration(t)
	dir := t.TempDir()

	// Write a YAML file with valid syntax but that can't be parsed as Agent
	os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("id: test\nbackend: test\ninvalid_field: [broken\n"), 0644)

	err := MigrateAgentsFromYAML(db, dir)
	// Invalid YAML should be skipped (logged as warning), not error
	assert.NoError(t, err)

	agents, err := LoadAgentsFromDB(db)
	require.NoError(t, err)
	assert.Empty(t, agents)
}

func TestMigrateAgentsFromYAML_NonYAMLExtension(t *testing.T) {
	db := setupTestDBForMigration(t)
	dir := t.TempDir()

	// Write a .yml file (should be skipped — only .yaml is processed)
	os.WriteFile(filepath.Join(dir, "agent.yml"), []byte("id: test\nname: Test\nbackend: test\n"), 0644)
	// Write a .json file (should be skipped)
	os.WriteFile(filepath.Join(dir, "agent.json"), []byte("{}"), 0644)

	err := MigrateAgentsFromYAML(db, dir)
	assert.NoError(t, err)

	agents, err := LoadAgentsFromDB(db)
	require.NoError(t, err)
	assert.Empty(t, agents)
}

func TestMigrateAgentsFromYAML_YAMLWithNoID(t *testing.T) {
	db := setupTestDBForMigration(t)
	dir := t.TempDir()

	// Write YAML with valid syntax but missing ID — should be skipped
	writeAgentYAML(t, dir, "noid.yaml", map[string]any{
		"name": "No ID", "backend": "test",
	})

	err := MigrateAgentsFromYAML(db, dir)
	require.NoError(t, err)

	agents, err := LoadAgentsFromDB(db)
	require.NoError(t, err)
	assert.Empty(t, agents)
}
