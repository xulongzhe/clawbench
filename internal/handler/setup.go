package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"clawbench/internal/model"
	"clawbench/internal/service"
	"clawbench/internal/summarize"
)

// setupCompleteMu prevents duplicate agent creation from concurrent requests.
var setupCompleteMu sync.Mutex

// ---------- GET /api/setup/status ----------

// ServeSetupStatus returns whether the setup wizard is needed.
// Response: { needs_setup, embedded_agent, agent_version }
func ServeSetupStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeLocalizedErrorf(w, r, http.StatusMethodNotAllowed, "MethodNotAllowed")
		return
	}

	needsSetup := len(model.AgentList) == 0
	embeddedPath := model.EmbeddedAgentPath()
	embeddedAgent := embeddedPath != ""
	agentVersion := model.EmbeddedAgentVersion()

	writeJSON(w, http.StatusOK, map[string]any{
		"needs_setup":    needsSetup,
		"embedded_agent": embeddedAgent,
		"agent_version":  agentVersion,
	})
}

// ---------- GET /api/setup/providers ----------

// ServeSetupProviders returns the list of providers that can be configured
// via the setup wizard. Enterprise providers (WizardReady=false) are excluded.
func ServeSetupProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeLocalizedErrorf(w, r, http.StatusMethodNotAllowed, "MethodNotAllowed")
		return
	}

	providers := model.GetWizardProviders()
	result := make([]map[string]string, 0, len(providers))
	for _, p := range providers {
		result = append(result, map[string]string{
			"id":     p.ID,
			"name":   p.Name,
			"envVar": p.EnvVar,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"providers":            result,
		"custom_url_supported": true,
	})
}

// ---------- POST /api/setup/models ----------

// setupModelsRequest is the request body for POST /api/setup/models.
type setupModelsRequest struct {
	Provider  string `json:"provider"`
	CustomURL string `json:"custom_url"`
	APIKey    string `json:"api_key"`
}

// ServeSetupModels lists available models for the selected provider.
// For providers with ModelsEndpoint: calls /v1/models via HTTP.
// For providers with KnownModels (Anthropic-format): returns hardcoded list.
func ServeSetupModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeLocalizedErrorf(w, r, http.StatusMethodNotAllowed, "MethodNotAllowed")
		return
	}

	var req setupModelsRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	// Custom URL mode: default provider to "openai" (OpenAI-compatible API format)
	if req.Provider == "" {
		if req.CustomURL == "" {
			writeLocalizedErrorf(w, r, http.StatusBadRequest, "InvalidRequestBody")
			return
		}
		req.Provider = "openai"
	}

	spec := model.FindProviderSpec(req.Provider)
	if spec == nil {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "ProviderNotFound")
		return
	}

	// Anthropic-format providers: return KnownModels
	if len(spec.KnownModels) > 0 && spec.ModelsEndpoint == "" {
		models := make([]map[string]any, 0, len(spec.KnownModels))
		for _, m := range spec.KnownModels {
			models = append(models, map[string]any{
				"id":                m.ID,
				"name":              m.Name,
				"created":           0,
				"context_length":    m.ContextLength,
				"supports_thinking": m.SupportsThinking,
				"cost_tier":         m.CostTier,
			})
		}
		hint := model.GetSummarizeModelHint(spec.KnownModels, nil)
		writeJSON(w, http.StatusOK, map[string]any{
			"models":               models,
			"summarize_model_hint": hint,
		})
		return
	}

	// OpenAI-compatible providers: call /v1/models endpoint
	modelsEndpoint := spec.ModelsEndpoint
	if req.CustomURL != "" {
		// Derive models URL from custom URL: replace last path segment with /models
		modelsEndpoint = deriveModelsURL(req.CustomURL)
	}

	if modelsEndpoint == "" {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "ModelsEndpointNotAvailable")
		return
	}

	models, err := fetchModelsFromEndpoint(modelsEndpoint, req.APIKey)
	if err != nil {
		slog.Warn("failed to fetch models from endpoint", "provider", req.Provider, "endpoint", modelsEndpoint, "error", err)
		writeJSON(w, http.StatusOK, map[string]any{
			"models":               []map[string]any{},
			"summarize_model_hint": "",
			"error":                err.Error(),
		})
		return
	}

	hint := model.GetSummarizeModelHint(nil, models)
	writeJSON(w, http.StatusOK, map[string]any{
		"models":               models,
		"summarize_model_hint": hint,
	})
}

// ---------- POST /api/setup/verify ----------

// setupVerifyRequest is the request body for POST /api/setup/verify.
type setupVerifyRequest struct {
	Provider  string `json:"provider"`
	CustomURL string `json:"custom_url"`
	APIKey    string `json:"api_key"`
	Model     string `json:"model"`
}

// ServeSetupVerify verifies the API key and model by sending a minimal test message
// through the Pi CLI. Returns success/failure with a message.
func ServeSetupVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeLocalizedErrorf(w, r, http.StatusMethodNotAllowed, "MethodNotAllowed")
		return
	}

	var req setupVerifyRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.APIKey == "" || req.Model == "" {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "InvalidRequestBody")
		return
	}

	// Custom URL mode: default provider to "openai" (OpenAI-compatible API format)
	if req.Provider == "" {
		req.Provider = "openai"
	}

	spec := model.FindProviderSpec(req.Provider)
	if spec == nil {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "ProviderNotFound")
		return
	}

	// Find the embedded Pi binary
	piPath := model.EmbeddedAgentPath()
	if piPath == "" {
		writeLocalizedErrorf(w, r, http.StatusNotFound, "EmbeddedAgentNotFound")
		return
	}

	// Build Pi CLI command
	args := []string{"-p", "--mode", "json", "--provider", req.Provider, "--model", req.Model}
	// Try --no-tools first (safer), fall back to --tools read if not supported
	args = append(args, "--no-tools", "ping")

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, piPath, args...)

	// Inject API key via environment variable
	envVar := spec.EnvVar
	if envVar != "" {
		cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s", envVar, req.APIKey))
	} else {
		cmd.Env = os.Environ()
	}

	// Inject custom URL if provided
	if req.CustomURL != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("PI_CUSTOM_URL=%s", req.CustomURL))
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := strings.TrimSpace(string(output))
		if errMsg == "" {
			errMsg = err.Error()
		}
		slog.Warn("setup verify failed", "provider", req.Provider, "model", req.Model, "error", err, "output", errMsg)

		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"message": "验证失败：API Key 无效或模型不可用。请检查后重试。",
			"model":   req.Model,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "配置验证成功！智能体工作正常。",
		"model":   req.Model,
	})
}

// ---------- POST /api/setup/complete ----------

// setupCompleteRequest is the request body for POST /api/setup/complete.
type setupCompleteRequest struct {
	Provider        string `json:"provider"`
	CustomURL       string `json:"custom_url"`
	APIKey          string `json:"api_key"`
	Model           string `json:"model"`
	SummarizeModel  string `json:"summarize_model"`
	AgentName       string `json:"agent_name"`
	AgentID         string `json:"agent_id"`
}

// ServeSetupComplete finalizes the setup wizard by creating the agent in the database,
// encrypting the API key, and writing Pi config files.
func ServeSetupComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeLocalizedErrorf(w, r, http.StatusMethodNotAllowed, "MethodNotAllowed")
		return
	}

	var req setupCompleteRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.APIKey == "" || req.Model == "" || req.AgentName == "" || req.AgentID == "" {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "InvalidRequestBody")
		return
	}

	// Custom URL mode: default provider to "openai" (OpenAI-compatible API format)
	if req.Provider == "" {
		if req.CustomURL == "" {
			writeLocalizedErrorf(w, r, http.StatusBadRequest, "InvalidRequestBody")
			return
		}
		req.Provider = "openai"
	}

	spec := model.FindProviderSpec(req.Provider)
	if spec == nil {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "ProviderNotFound")
		return
	}

	// Concurrency guard: prevent duplicate agent creation
	if !setupCompleteMu.TryLock() {
		writeLocalizedErrorf(w, r, http.StatusConflict, "Conflict")
		return
	}
	defer setupCompleteMu.Unlock()

	// Check for duplicate agent ID
	if _, exists := model.Agents[req.AgentID]; exists {
		writeLocalizedErrorf(w, r, http.StatusConflict, "AgentAlreadyExists")
		return
	}

	// Determine Pi binary path
	piPath := model.EmbeddedAgentPath()

	// 1. Write Pi config files (auth.json, settings.json) — best effort, don't block on failure
	if piPath != "" {
		writePiConfigFiles(req, spec)
	}

	// 2. Insert agent + API key in a DB transaction for atomicity
	tx, err := service.DB.Begin()
	if err != nil {
		slog.Error("failed to begin transaction", "error", err)
		writeLocalizedErrorf(w, r, http.StatusInternalServerError, "InternalError")
		return
	}
	defer tx.Rollback()

	agent := &model.Agent{
		ID:              req.AgentID,
		Name:            req.AgentName,
		Icon:            "🥧",
		Specialty:       "极简编程智能体",
		Backend:         "pi",
		Command:         piPath,
		PreferredModel:  req.Model,
		Source:          "setup",
		ModelsAutoDetected: false,
	}
	if err := service.SaveAgent(tx, agent); err != nil {
		slog.Error("failed to save agent to DB", "agent_id", req.AgentID, "error", err)
		writeLocalizedErrorf(w, r, http.StatusInternalServerError, "InternalError")
		return
	}

	// 3. Encrypt and store API key
	if err := service.SaveAgentAPIKey(tx, req.AgentID, req.Provider, req.CustomURL, req.APIKey); err != nil {
		slog.Error("failed to save API key", "agent_id", req.AgentID, "error", err)
		writeLocalizedErrorf(w, r, http.StatusInternalServerError, "InternalError")
		return
	}

	if err := tx.Commit(); err != nil {
		slog.Error("failed to commit transaction", "error", err)
		writeLocalizedErrorf(w, r, http.StatusInternalServerError, "InternalError")
		return
	}

	// 4. Auto-configure summarize backend
	if spec.ChatEndpoint != "" && req.SummarizeModel != "" {
		configureSummarizeBackend(req, spec)
	}

	// 5. Reload agents from DB into memory
	if err := service.LoadAgentsIntoMemory(service.DB); err != nil {
		slog.Error("failed to reload agents into memory", "error", err)
	}

	slog.Info("setup complete: agent created", "agent_id", req.AgentID, "provider", req.Provider, "model", req.Model)

	writeJSON(w, http.StatusOK, map[string]any{
		"success":          true,
		"agent":            model.Agents[req.AgentID],
		"default_agent_id": model.GetDefaultAgentID(),
	})
}

// configureSummarizeBackend auto-configures the summarize backend after wizard completion.
// Writes config.yaml with backend:api settings and reinitializes the global summarizer.
func configureSummarizeBackend(req setupCompleteRequest, spec *model.ProviderSpec) {
	// Write summarize config to config.yaml
	chatEndpoint := spec.ChatEndpoint
	if req.CustomURL != "" {
		chatEndpoint = req.CustomURL
	}

	summarizePatch := map[string]any{
		"summarize": map[string]any{
			"backend": "api",
			"model":   req.SummarizeModel,
			"api": map[string]any{
				"base_url": chatEndpoint,
				"key":      "", // key read from agent_api_keys at runtime
				"format":   spec.APIFormat,
				"agent_id": req.AgentID,
			},
		},
	}

	if err := writeConfigYAML(summarizePatch); err != nil {
		slog.Warn("failed to write summarize config to config.yaml", "error", err)
		// Non-fatal: summarize will fall back to SimpleSummarizer
	}

	// Update in-memory config
	model.ConfigInstance.Summarize.Backend = "api"
	model.ConfigInstance.Summarize.Model = req.SummarizeModel
	model.ConfigInstance.Summarize.API.BaseURL = chatEndpoint
	model.ConfigInstance.Summarize.API.Key = ""
	model.ConfigInstance.Summarize.API.Format = spec.APIFormat
	model.ConfigInstance.Summarize.API.AgentID = req.AgentID

	// Reinitialize the global summarizer at runtime
	reinitSummarizer(req, spec)
}

// reinitSummarizer creates a new OpenAISummarizer/AnthropicSummarizer from the
// provider registry and decrypted API key, and updates the global summarizer.
func reinitSummarizer(req setupCompleteRequest, spec *model.ProviderSpec) {
	chatEndpoint := spec.ChatEndpoint
	if req.CustomURL != "" {
		chatEndpoint = req.CustomURL
	}

	// API key already stored in agent_api_keys table — read it back
	customURL, apiKey, err := service.LoadAgentAPIKey(service.DB, req.AgentID, req.Provider)
	if err != nil {
		slog.Warn("failed to load API key for summarize reinit", "error", err)
		return
	}
	_ = customURL // not needed for summarize

	if apiKey == "" {
		slog.Warn("no API key available for summarize reinit")
		return
	}

	// Create the appropriate summarizer based on API format
	var newSummarizer summarize.Summarizer
	switch spec.APIFormat {
	case "openai":
		newSummarizer = summarize.NewOpenAI(chatEndpoint, apiKey, req.SummarizeModel)
	case "anthropic":
		newSummarizer = summarize.NewAnthropic(chatEndpoint, apiKey, req.SummarizeModel)
	default:
		slog.Warn("unknown API format for summarize reinit", "format", spec.APIFormat)
		return
	}

	SetSummarizer(newSummarizer)
	slog.Info("reinitialized summarize backend", "provider", req.Provider, "model", req.SummarizeModel, "format", spec.APIFormat)
}

// ---------- Helper functions ----------

// deriveModelsURL replaces the last path segment of a URL with "models".
// E.g., "https://api.example.com/v1/chat/completions" → "https://api.example.com/v1/models"
func deriveModelsURL(baseURL string) string {
	lastSlash := strings.LastIndex(baseURL, "/")
	if lastSlash <= 0 {
		return ""
	}
	parent := baseURL[:lastSlash]
	return parent + "/models"
}

// fetchModelsFromEndpoint calls the OpenAI-compatible /v1/models endpoint
// and returns the parsed model list.
func fetchModelsFromEndpoint(endpoint, apiKey string) ([]model.ModelInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("models endpoint returned %d", resp.StatusCode)
	}

	// Parse OpenAI /v1/models response
	var result struct {
		Data []struct {
			ID      string `json:"id"`
			Name    string `json:"name,omitempty"`
			Created int64  `json:"created"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse models response: %w", err)
	}

	models := make([]model.ModelInfo, 0, len(result.Data))
	for _, m := range result.Data {
		name := m.Name
		if name == "" {
			name = m.ID
		}
		models = append(models, model.ModelInfo{
			ID:      m.ID,
			Name:    name,
			Created: m.Created,
		})
	}

	return models, nil
}

// writePiConfigFiles writes Pi CLI configuration files (auth.json, settings.json).
// These are best-effort writes — failures are logged but don't block setup completion.
func writePiConfigFiles(req setupCompleteRequest, spec *model.ProviderSpec) {
	// Determine Pi config directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		slog.Warn("failed to get home dir for Pi config", "error", err)
		return
	}
	piConfigDir := filepath.Join(homeDir, ".pi", "agent")
	if err := os.MkdirAll(piConfigDir, 0755); err != nil {
		slog.Warn("failed to create Pi config dir", "dir", piConfigDir, "error", err)
		return
	}

	// Write auth.json
	authData := map[string]string{
		spec.EnvVar: req.APIKey,
	}
	authJSON, _ := json.Marshal(authData)
	if err := atomicWriteFile(filepath.Join(piConfigDir, "auth.json"), authJSON, 0600); err != nil {
		slog.Warn("failed to write Pi auth.json", "error", err)
	}

	// Write settings.json
	settingsData := map[string]string{
		"defaultProvider": req.Provider,
		"defaultModel":   req.Model,
	}
	settingsJSON, _ := json.Marshal(settingsData)
	if err := atomicWriteFile(filepath.Join(piConfigDir, "settings.json"), settingsJSON, 0644); err != nil {
		slog.Warn("failed to write Pi settings.json", "error", err)
	}
}

// atomicWriteFile writes data to a file atomically (write to .tmp, then rename).
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, perm); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
