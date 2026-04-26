package handler

import (
	"net/http"

	"clawbench/internal/model"
)

// ServeAgents returns the list of configured AI agents.
func ServeAgents(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"agents": model.AgentList,
	})
}
