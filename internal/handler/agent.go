package handler

import (
	"encoding/json"
	"net/http"

	"clawbench/internal/model"
)

// ServeAgents returns the list of configured AI agents.
func ServeAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	json.NewEncoder(w).Encode(map[string]any{
		"agents": model.AgentList,
	})
}
