package model

import "testing"

// TestWriteAgentYAML_NotInitialized_Internal tests the ConfigDir == "" path
// which is only reachable from within the package.
func TestWriteAgentYAML_NotInitialized_Internal(t *testing.T) {
	// Save and restore
	origAgents := Agents
	origList := AgentList
	origDir := ConfigDir
	t.Cleanup(func() {
		Agents = origAgents
		AgentList = origList
		ConfigDir = origDir
	})

	// Reset to zero state (ConfigDir == "")
	Agents = nil
	AgentList = nil
	ConfigDir = ""

	agent := &Agent{ID: "test"}
	err := WriteAgentYAML(agent)
	if err == nil {
		t.Error("expected error when ConfigDir is not initialized")
	}
}
