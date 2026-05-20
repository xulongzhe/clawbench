package model

import "testing"

// TestWriteAgentYAML_NotInitialized_Internal tests the agentsDir == "" path
// which is only reachable from within the package.
func TestWriteAgentYAML_NotInitialized_Internal(t *testing.T) {
	// Save and restore
	origAgents := Agents
	origList := AgentList
	origDir := agentsDir
	t.Cleanup(func() {
		Agents = origAgents
		AgentList = origList
		agentsDir = origDir
	})

	// Reset to zero state (agentsDir == "")
	Agents = nil
	AgentList = nil
	agentsDir = ""

	agent := &Agent{ID: "test"}
	err := WriteAgentYAML(agent)
	if err == nil {
		t.Error("expected error when agentsDir is not initialized")
	}
}
