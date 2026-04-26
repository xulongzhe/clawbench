package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"clawbench/internal/ai"
	"clawbench/internal/model"
	"clawbench/internal/platform"
	"clawbench/internal/service"
)

const maxChatBodySize = 10 << 20 // 10MB

// ServeAISession handles DELETE for Claude CLI internal session files.
func ServeAISession(w http.ResponseWriter, r *http.Request) {
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	if !requireMethod(w, r, http.MethodDelete) {
		return
	}

	// Get Claude session directory using cross-platform path mangling
	sessionDir := platform.ClaudeProjectDir(projectPath)

	// Delete all .jsonl session files
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		// Session dir doesn't exist — nothing to delete, treat as success
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "deleted": 0})
		return
	}

	deleted := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jsonl") {
			if err := os.Remove(filepath.Join(sessionDir, entry.Name())); err == nil {
				deleted++
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "deleted": deleted})
}

// AIChat handles GET (status/history) and POST (send message) for AI chat.
func AIChat(w http.ResponseWriter, r *http.Request) {
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	// GET: return full chat history + running status
	if r.Method == http.MethodGet {
		// Check if a specific session is requested
		requestedSessionID := r.URL.Query().Get("session_id")

		var sessionID string
		var sessionBackend string

		if requestedSessionID != "" {
			// Use the requested session
			sessionID = requestedSessionID
			sessionBackend = service.GetSessionBackend(sessionID)
			if sessionBackend == "" {
				model.WriteErrorf(w, http.StatusNotFound, "session not found")
				return
			}
		} else {
			// No specific session requested, get the most recent session across all backends
			allSessions, err := service.GetSessions(projectPath, "")
			if err != nil {
				model.WriteError(w, model.Internal(fmt.Errorf("failed to load sessions")))
				return
			}

			if len(allSessions) == 0 {
				// No sessions exist, create a new one with default agent
				agentID := model.GetDefaultAgentID()
				sessionBackend2, defaultModel, _, _, ok := resolveAgentConfig(agentID)
				if !ok {
					model.WriteErrorf(w, http.StatusServiceUnavailable, "no agents available")
					return
				}
				sessionID, err = service.CreateSession(projectPath, sessionBackend2, "新会话", agentID, defaultModel)
				if err != nil {
					model.WriteError(w, model.Internal(fmt.Errorf("failed to create session")))
					return
				}
			} else {
				// Use the most recent session (already sorted by updated_at DESC)
				sessionID = allSessions[0].ID
				sessionBackend = allSessions[0].Backend
			}
		}

		// Always update cookie with current session ID
		setSessionID(w, sessionID)
		// Mark session as read
		service.UpdateLastRead(sessionID)

		// Parse pagination params
		limit := 0
		beforeTime := ""
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
				limit = l
			}
		}
		if bt := r.URL.Query().Get("before"); bt != "" {
			beforeTime = bt
		}

		// If limit not specified, use config default
		if limit == 0 {
			limit = model.ChatInitialMessages
		}
		// Cap limit to prevent abuse
		if limit > 100 {
			limit = 100
		}

		totalCount := service.GetChatMessageCount(sessionID)
		messages, err := service.GetChatHistoryPaged(projectPath, sessionBackend, sessionID, limit, beforeTime)
		// Get session title and agent info
		sessionTitle, _ := service.GetSessionTitle(sessionID)
		sessionAgentID := service.GetSessionAgentID(sessionID)
		running := service.IsSessionRunning(sessionID)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"messages": []any{}, "running": running, "sessionId": sessionID, "sessionTitle": sessionTitle, "backend": sessionBackend, "agentId": sessionAgentID, "total": totalCount})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"messages": messages, "running": running, "sessionId": sessionID, "sessionTitle": sessionTitle, "backend": sessionBackend, "agentId": sessionAgentID, "total": totalCount})
		return
	}

	if r.Method != http.MethodPost {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get backend from session, not from global state
	sessionID := getSessionID(r)
	if sessionID == "" {
		// No session yet — auto-create one (same logic as GET)
		agentID2 := model.GetDefaultAgentID()
		sessionBackend2, defaultModel2, _, _, ok := resolveAgentConfig(agentID2)
		if !ok {
			model.WriteErrorf(w, http.StatusServiceUnavailable, "no agents available")
			return
		}
		var err error
		sessionID, err = service.CreateSession(projectPath, sessionBackend2, "新会话", agentID2, defaultModel2)
		if err != nil {
			model.WriteError(w, model.Internal(fmt.Errorf("failed to create session")))
			return
		}
		setSessionID(w, sessionID)
	}
	backendName := service.GetSessionBackend(sessionID)
	if backendName == "" {
		model.WriteErrorf(w, http.StatusBadRequest, "Session backend not found")
		return
	}

	// Prevent concurrent sessions for the same session ID
	if !service.TrySetSessionRunning(sessionID) {
		writeJSON(w, http.StatusOK, map[string]any{"running": true})
		return
	}

	var req struct {
		Message   string   `json:"message"`
		FilePath  string   `json:"filePath"`    // legacy: single file path
		FilePaths []string `json:"filePaths"`   // new: multiple file paths
		Files     []string `json:"files"`
		AgentID   string   `json:"agentId"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxChatBodySize)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		service.SetSessionRunning(sessionID, false)
		model.WriteErrorf(w, http.StatusBadRequest, "Invalid request")
		return
	}

	// Allow empty message if files are provided
	if req.Message == "" && len(req.Files) == 0 && len(req.FilePaths) == 0 {
		service.SetSessionRunning(sessionID, false)
		model.WriteErrorf(w, http.StatusBadRequest, "Message or files required")
		return
	}

	// Merge legacy filePath into filePaths for unified handling
	allFilePaths := make([]string, 0, len(req.FilePaths)+1)
	if req.FilePath != "" {
		allFilePaths = append(allFilePaths, req.FilePath)
	}
	for _, p := range req.FilePaths {
		if p != req.FilePath { // dedup
			allFilePaths = append(allFilePaths, p)
		}
	}

	basePath, _ := filepath.Abs(projectPath)
	var fileDir string
	if len(allFilePaths) > 0 {
		firstAbsPath, ok := validateAndResolvePath(w, basePath, allFilePaths[0])
		if !ok {
			service.SetSessionRunning(sessionID, false)
			return
		}
		if _, err := os.Stat(firstAbsPath); err != nil {
			service.SetSessionRunning(sessionID, false)
			model.WriteError(w, model.NotFound(nil, "File not found"))
			return
		}
		fileDir = filepath.Dir(firstAbsPath)
	} else {
		fileDir = basePath
	}

	// Validate all attached file paths are within project
	validatedFilePaths := make([]string, 0, len(allFilePaths))
	for _, fp := range allFilePaths {
		fAbsPath, ok := validateAndResolvePath(w, basePath, fp)
		if !ok {
			service.SetSessionRunning(sessionID, false)
			return
		}
		if _, err := os.Stat(fAbsPath); err != nil {
			service.SetSessionRunning(sessionID, false)
			model.WriteError(w, model.NotFound(nil, "File not found: "+fp))
			return
		}
		validatedFilePaths = append(validatedFilePaths, fAbsPath)
	}

	// Validate file paths are within project and collect absolute paths
	fileAbsPaths := make([]string, 0, len(req.Files))
	for _, fPath := range req.Files {
		fAbsPath, ok := validateAndResolvePath(w, basePath, fPath)
		if !ok {
			service.SetSessionRunning(sessionID, false)
			return
		}
		if _, err := os.Stat(fAbsPath); err != nil {
			service.SetSessionRunning(sessionID, false)
			model.WriteError(w, model.NotFound(nil, "File not found: "+fPath))
			return
		}
		fileAbsPaths = append(fileAbsPaths, fAbsPath)
	}

	prompt := req.Message
	if len(validatedFilePaths) > 0 {
		prompt = fmt.Sprintf("[当前文件: %s]\n%s", strings.Join(validatedFilePaths, ", "), req.Message)
	}
	if len(fileAbsPaths) > 0 {
		prompt = fmt.Sprintf("[用户上传了 %d 个文件: %s]\n%s", len(fileAbsPaths), strings.Join(fileAbsPaths, ", "), prompt)
	}

	// For DB storage: use first filePath for legacy column, rest go into files
	legacyFilePath := ""
	if len(allFilePaths) > 0 {
		legacyFilePath = allFilePaths[0]
	}
	// Merge remaining filePaths into files for storage
	allFiles := req.Files
	if len(allFilePaths) > 1 {
		allFiles = append(allFiles, allFilePaths[1:]...)
	}
	if _, err := service.AddChatMessage(projectPath, backendName, sessionID, "user", req.Message, legacyFilePath, allFiles, false); err != nil {
		service.SetSessionRunning(sessionID, false)
		model.WriteError(w, model.Internal(fmt.Errorf("failed to save message")))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"started": true, "sessionId": sessionID})

	// Register stream channel BEFORE starting goroutine to avoid race with SSE connection
	streamCh := service.RegisterSessionStream(sessionID)

	slog.Info("about to start ai goroutine", slog.String("project", projectPath))

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("AI goroutine panicked",
					slog.String("session", sessionID),
					slog.Any("panic", r),
					slog.String("stack", string(debug.Stack())),
				)
				service.SetSessionRunning(sessionID, false)
				service.UnregisterSessionCancel(sessionID)
				// Try to send error event to SSE stream
				service.SendSessionEvent(sessionID, ai.StreamEvent{Type: "error", Error: "AI 内部错误，请重试"})
				service.UnregisterSessionStream(sessionID)
				// Persist error to database
				errMsg := "AI 内部错误，请重试"
				errContent, _ := json.Marshal(map[string]any{"blocks": []any{map[string]string{"type": "error", "text": errMsg}}})
				service.FinalizeStreamingMessage(projectPath, backendName, sessionID, string(errContent))
			}
		}()
		slog.Info("ai goroutine started", slog.String("project", projectPath))
		defer service.SetSessionRunning(sessionID, false)
		defer service.UnregisterSessionStream(sessionID)

		slog.Info("ai stream request started",
			slog.String("backend", backendName),
			slog.String("session", sessionID),
			slog.String("work_dir", fileDir),
		)

		// Resolve agent config for system prompt and model override
		agentID := req.AgentID
		systemPrompt := ""
		agentModel := ""
		agentCommand := ""

		if agentID == "" {
			agentID = model.GetDefaultAgentID()
		}
		if agentID == "" {
			model.WriteErrorf(w, http.StatusServiceUnavailable, "no agents available")
			return
		}
		if agent, ok := model.Agents[agentID]; ok {
			systemPrompt = agent.SystemPrompt
			if agent.Model != "" {
				agentModel = agent.Model
			}
			if agent.Command != "" {
				agentCommand = agent.Command
			}
		}

		// For OpenCode/Codex backends, resolve external session ID when resuming
		effectiveSessionID := sessionID
		resume := service.SessionHasAssistant(sessionID)
		if (backendName == "opencode" || backendName == "codex") && resume {
			extID := service.GetExternalSessionID(sessionID)
			if extID != "" {
				effectiveSessionID = extID
			}
		}

		chatReq := ai.ChatRequest{
			Prompt:       prompt,
			SessionID:    effectiveSessionID,
			WorkDir:      fileDir,
			SystemPrompt: systemPrompt,
			Model:        agentModel,
			Command:      agentCommand,
			AgentID:      agentID,
			Resume:       resume,
		}

		// Use independent context with cancel to prevent goroutine leaks
		// and support user-initiated cancellation (no timeout - let AI run indefinitely)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		service.RegisterSessionCancel(sessionID, cancel)
		defer service.UnregisterSessionCancel(sessionID)

		backend, err := ai.NewBackend(backendName)
		if err != nil {
			slog.Error("failed to create backend", slog.String("backend", backendName), slog.String("err", err.Error()))
			errMsg := fmt.Sprintf("创建 AI Backend 失败: %v", err)
			if !sendEvent(ctx, streamCh, ai.StreamEvent{Type: "error", Error: errMsg}) {
				return
			}
			_, _ = service.AddChatMessage(projectPath, backendName, sessionID, "assistant", errMsg, "", nil, false)
			return
		}

		eventCh, err := backend.ExecuteStream(ctx, chatReq)
		if err != nil {
			slog.Error("failed to start stream", slog.String("err", err.Error()))
			errMsg := fmt.Sprintf("启动流式输出失败: %v", err)
			if !sendEvent(ctx, streamCh, ai.StreamEvent{Type: "error", Error: errMsg}) {
				return
			}
			_, _ = service.AddChatMessage(projectPath, backendName, sessionID, "assistant", errMsg, "", nil, false)
			return
		}

		// Create streaming placeholder message in DB
		emptyContent, _ := json.Marshal(map[string]any{"blocks": []any{}})
		_, _ = service.AddChatMessage(projectPath, backendName, sessionID, "assistant", string(emptyContent), "", nil, true)

		var blocks []model.ContentBlock
		var responseMetadata *ai.Metadata

		// Incremental persistence: flush every 1s or every 5 events
		flushTicker := time.NewTicker(1 * time.Second)
		defer flushTicker.Stop()
		eventCount := 0

		serializeBlocks := func() string {
			contentMap := map[string]any{"blocks": blocks}
			if responseMetadata != nil {
				contentMap["metadata"] = responseMetadata
			}
			blocksJSON, _ := json.Marshal(contentMap)
			return string(blocksJSON)
		}

		for {
			select {
			case event, ok := <-eventCh:
				if !ok {
					// Stream ended — flush ticker and finalize below
					goto finalize
				}
				// Don't forward "done" here — send it after DB finalize
				// so that the frontend loads complete content on "done".
				if event.Type == "done" {
					// Record that stream completed normally (not cancelled/errored)
					// The actual "done" SSE event is sent after FinalizeStreamingMessage.
					goto finalize
				}
				// Forward to SSE channel
				if !sendEvent(ctx, streamCh, event) {
					goto finalize
				}

				accumulateBlock(&blocks, event)
				if event.Type == "metadata" && event.Meta != nil {
					responseMetadata = event.Meta
					// Capture external session ID on first response (OpenCode/Codex)
					if (backendName == "opencode" || backendName == "codex") && event.Meta.SessionID != "" {
						existingExtID := service.GetExternalSessionID(sessionID)
						if existingExtID == "" {
							if err := service.UpdateExternalSessionID(sessionID, event.Meta.SessionID); err != nil {
								slog.Error("failed to save external session ID",
									slog.String("session", sessionID),
									slog.String("external_id", event.Meta.SessionID),
									slog.String("err", err.Error()),
								)
							}
						}
					}
				}
				eventCount++
				if eventCount%5 == 0 {
					if err := service.UpdateStreamingMessage(projectPath, backendName, sessionID, serializeBlocks()); err != nil {
						slog.Error("failed to update streaming message",
							slog.String("session", sessionID),
							slog.String("err", err.Error()),
						)
					}
				}
			case <-flushTicker.C:
				if len(blocks) > 0 {
					if err := service.UpdateStreamingMessage(projectPath, backendName, sessionID, serializeBlocks()); err != nil {
						slog.Error("failed to update streaming message",
							slog.String("session", sessionID),
							slog.String("err", err.Error()),
						)
					}
				}
			}
		}

	finalize:
		// Determine cancellation reason
		cancelReason := service.GetAndClearCancelReason(sessionID)

		// Serialize blocks + metadata as JSON for database storage
		var content string
		if len(blocks) == 0 {
			// Auto-infer reason for empty response
			var errMsg string
			switch {
			case cancelReason == "user":
				errMsg = "用户已中断"
			case cancelReason == "disconnect":
				errMsg = "连接已断开，AI 响应中断"
			case ctx.Err() == context.Canceled:
				errMsg = "AI 响应被中断"
			case ctx.Err() == context.DeadlineExceeded:
				errMsg = "AI 响应超时（30分钟）"
			default:
				errMsg = "AI 未返回任何内容"
			}
			blocks = append(blocks, model.ContentBlock{Type: "warning", Text: errMsg})
			contentMap := map[string]any{"blocks": blocks}
			if cancelReason == "user" || cancelReason == "disconnect" || ctx.Err() == context.Canceled {
				contentMap["cancelled"] = true
			}
			blocksJSON, _ := json.Marshal(contentMap)
			content = string(blocksJSON)
		} else {
			contentMap := map[string]any{"blocks": blocks}
			if responseMetadata != nil {
				contentMap["metadata"] = responseMetadata
			}
			// When there are blocks but the stream was interrupted, add a warning and mark cancelled
			if cancelReason == "disconnect" {
				blocks = append(blocks, model.ContentBlock{Type: "warning", Text: "连接已断开，AI 响应中断"})
				contentMap["cancelled"] = true
			} else if cancelReason == "user" || ctx.Err() == context.Canceled {
				contentMap["cancelled"] = true
			} else if ctx.Err() == context.DeadlineExceeded {
				blocks = append(blocks, model.ContentBlock{Type: "warning", Text: "AI 响应超时（30分钟）"})
			}
			contentMap["blocks"] = blocks
			blocksJSON, _ := json.Marshal(contentMap)
			content = string(blocksJSON)
		}
		if err := service.FinalizeStreamingMessage(projectPath, backendName, sessionID, content); err != nil {
			slog.Error("failed to finalize streaming message",
				slog.String("session", sessionID),
				slog.String("err", err.Error()),
			)
		}

		// Send terminal SSE event AFTER DB finalize, so frontend can safely
		// load complete content from DB when it receives "done"/"cancelled".
		if cancelReason == "user" || cancelReason == "disconnect" || ctx.Err() == context.Canceled {
			sendEvent(ctx, streamCh, ai.StreamEvent{Type: "cancelled"})
		} else if ctx.Err() == context.DeadlineExceeded {
			sendEvent(ctx, streamCh, ai.StreamEvent{Type: "error", Error: "AI 响应超时（30分钟）"})
		} else if len(blocks) == 0 {
			sendEvent(ctx, streamCh, ai.StreamEvent{Type: "error", Error: "AI 未返回任何内容"})
		} else {
			// Normal completion
			sendEvent(ctx, streamCh, ai.StreamEvent{Type: "done"})
		}

		slog.Info("ai stream request done",
			slog.String("session", sessionID),
			slog.Int("blocks", len(blocks)),
			slog.String("cancel_reason", cancelReason),
		)
	}()
}

// CancelChat handles POST to cancel an ongoing AI stream for a session.
func CancelChat(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		sessionID = getSessionID(r)
	}
	if sessionID == "" {
		model.WriteErrorf(w, http.StatusBadRequest, "session_id required")
		return
	}

	if !service.CancelSession(sessionID) {
		model.WriteErrorf(w, http.StatusNotFound, "session not running")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// accumulateBlock processes a single StreamEvent and updates the blocks slice.
// Both text and thinking events are coalesced into the most recent block of
// the same type; tool_use events are deduplicated by ID.
//
// When AI models (e.g. GLM-5.1) interleave thinking_delta and text_delta events,
// the last block may not be the same type as the incoming event. Instead of only
// checking the last block, we search backward for the most recent block of the
// same type and merge into it. However, tool_use blocks act as natural boundaries —
// text/thinking after a tool_use should not be merged with text/thinking before it.
// This prevents a single thinking or text block from being fragmented into many
// tiny blocks when events alternate, while preserving the semantic separation
// around tool calls.
func accumulateBlock(blocks *[]model.ContentBlock, event ai.StreamEvent) {
	// findLastBlockOfType searches backward for the most recent block of the
	// given type, but stops at tool_use boundaries (they are natural separators).
	findLastBlockOfType := func(typ string) (int, bool) {
		for i := len(*blocks) - 1; i >= 0; i-- {
			if (*blocks)[i].Type == typ {
				return i, true
			}
			// tool_use blocks are natural boundaries — don't merge across them
			if (*blocks)[i].Type == "tool_use" {
				return -1, false
			}
		}
		return -1, false
	}

	switch event.Type {
	case "content":
		// Coalesce incremental content deltas into the most recent text block.
		if idx, found := findLastBlockOfType("text"); found {
			(*blocks)[idx].Text += event.Content
		} else {
			*blocks = append(*blocks, model.ContentBlock{Type: "text", Text: event.Content})
		}
	case "thinking":
		// Coalesce incremental thinking deltas into the most recent thinking block.
		if idx, found := findLastBlockOfType("thinking"); found {
			(*blocks)[idx].Text += event.Content
		} else {
			*blocks = append(*blocks, model.ContentBlock{Type: "thinking", Text: event.Content})
		}
	case "tool_use":
		if event.Tool != nil {
			// Parse tool input JSON into map
			var input map[string]any
			if event.Tool.Input != "" {
				json.Unmarshal([]byte(event.Tool.Input), &input)
			}
			if input == nil {
				input = make(map[string]any)
			}
			// Find existing block by tool ID and update, or append new
			found := false
			for i := len(*blocks) - 1; i >= 0; i-- {
				if (*blocks)[i].Type == "tool_use" && (*blocks)[i].ID == event.Tool.ID {
					if len(input) > 0 {
						(*blocks)[i].Input = input
					}
					found = true
					break
				}
			}
			if !found {
				*blocks = append(*blocks, model.ContentBlock{
					Type:  "tool_use",
					Name:  event.Tool.Name,
					ID:    event.Tool.ID,
					Input: input,
				})
			}
		}
	case "warning":
		*blocks = append(*blocks, model.ContentBlock{Type: "warning", Text: event.Content})
	case "error":
		*blocks = append(*blocks, model.ContentBlock{Type: "warning", Text: event.Error})
	}
}

// sendEvent sends an event to the stream channel.
// Non-blocking: if the channel is full (no SSE client reading), the event is dropped.
// This is safe because content is persisted to DB independently.
func sendEvent(ctx context.Context, ch chan<- ai.StreamEvent, event ai.StreamEvent) bool {
	select {
	case ch <- event:
		return true
	case <-ctx.Done():
		return false
	default:
		// Channel full — drop the event, DB persistence ensures no data loss
		return true
	}
}
