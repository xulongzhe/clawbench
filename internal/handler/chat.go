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
	"strings"
	"time"

	"clawbench/internal/ai"
	"clawbench/internal/model"
	"clawbench/internal/platform"
	"clawbench/internal/service"
)

const maxChatBodySize = 10 << 20 // 10MB

// ServeChatHistory handles GET (list), POST (add), DELETE (clear) for chat history.
func ServeChatHistory(w http.ResponseWriter, r *http.Request) {
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		sessionID := r.URL.Query().Get("session_id")
		if sessionID == "" {
			// Get current session from cookie or create default
			sessionID = getSessionID(r)
			if sessionID == "" {
				// Try to get or create default session
				sessions, err := service.GetSessions(projectPath, "")
				if err != nil {
					model.WriteError(w, model.Internal(fmt.Errorf("failed to load sessions")))
					return
				}
				if len(sessions) == 0 {
					// Create default session with default agent
					defaultAgentID := "assistant"
					var backend, defaultModel string
					if agent, ok := model.Agents[defaultAgentID]; ok && agent != nil {
						backend = agent.Backend
						defaultModel = agent.Model
					}
					if backend == "" {
						backend = "codebuddy"
					}
					if defaultModel == "" {
						defaultModel = "glm-5.1"
					}
					sessionID, err = service.CreateSession(projectPath, backend, "新会话", defaultAgentID, defaultModel)
					if err != nil {
						model.WriteError(w, model.Internal(fmt.Errorf("failed to create session")))
						return
					}
				} else {
					sessionID = sessions[0].ID
				}
				// Set session cookie
				setSessionID(w, sessionID)
			}
		}
		backend := service.GetSessionBackend(sessionID)
		if backend == "" {
			model.WriteErrorf(w, http.StatusNotFound, "session not found")
			return
		}
		messages, err := service.GetChatHistory(projectPath, backend, sessionID)
		if err != nil {
			model.WriteError(w, model.Internal(fmt.Errorf("failed to load history")))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"messages": messages, "sessionId": sessionID})

	case http.MethodPost:
		var req struct {
			Role     string   `json:"role"`
			Content  string   `json:"content"`
			FilePath string   `json:"file_path"`
			Files    []string `json:"files"`
			SessionID string   `json:"session_id"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxChatBodySize)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			model.WriteErrorf(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		if req.Role != "user" && req.Role != "assistant" {
			model.WriteErrorf(w, http.StatusBadRequest, "Invalid role")
			return
		}
		sessionID := req.SessionID
		if sessionID == "" {
			sessionID = getSessionID(r)
		}
		backend := service.GetSessionBackend(sessionID)
		if backend == "" {
			model.WriteErrorf(w, http.StatusBadRequest, "session not found")
			return
		}
		if _, err := service.AddChatMessage(projectPath, backend, sessionID, req.Role, req.Content, req.FilePath, req.Files, false); err != nil {
			model.WriteError(w, model.Internal(fmt.Errorf("failed to save message")))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "savedAt": time.Now().UTC()})

	default:
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// ServeAISession handles DELETE for Claude CLI internal session files.
func ServeAISession(w http.ResponseWriter, r *http.Request) {
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	if r.Method != http.MethodDelete {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get Claude session directory using cross-platform path mangling
	sessionDir := platform.ClaudeProjectDir(projectPath)

	// Delete all .jsonl session files
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		// Session dir doesn't exist — nothing to delete, treat as success
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "deleted": 0})
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "deleted": deleted})
}

// ServeChatCount returns the message count for a session (lightweight polling endpoint).
func ServeChatCount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		model.WriteErrorf(w, http.StatusBadRequest, "session_id required")
		return
	}
	count := service.GetChatMessageCount(sessionID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"count": count})
}

// ServeChatMessageUpdate handles PUT to update a specific message's content.
func ServeChatMessageUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var req struct {
		MessageID int64  `json:"messageId"`
		Content   string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		model.WriteErrorf(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.MessageID == 0 {
		model.WriteErrorf(w, http.StatusBadRequest, "messageId required")
		return
	}
	if err := service.UpdateMessageContent(int(req.MessageID), req.Content); err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("failed to update message")))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
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
				defaultAgentID := "assistant"
				var sessionBackend, defaultModel string
				if agent, ok := model.Agents[defaultAgentID]; ok && agent != nil {
					sessionBackend = agent.Backend
					defaultModel = agent.Model
				}
				if sessionBackend == "" {
					sessionBackend = "codebuddy"
				}
				if defaultModel == "" {
					defaultModel = "glm-5.1"
				}
				sessionID, err = service.CreateSession(projectPath, sessionBackend, "新会话", defaultAgentID, defaultModel)
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
		messages, err := service.GetChatHistory(projectPath, sessionBackend, sessionID)
		// Get session title and agent info
		sessionTitle, _ := service.GetSessionTitle(sessionID)
		sessionAgentID := service.GetSessionAgentID(sessionID)
		w.Header().Set("Content-Type", "application/json")
		running := service.IsSessionRunning(sessionID)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]any{"messages": []any{}, "running": running, "sessionId": sessionID, "sessionTitle": sessionTitle, "backend": sessionBackend, "agentId": sessionAgentID})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"messages": messages, "running": running, "sessionId": sessionID, "sessionTitle": sessionTitle, "backend": sessionBackend, "agentId": sessionAgentID})
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
		defaultAgentID := "assistant"
		var sessionBackend, defaultModel string
		if agent, ok := model.Agents[defaultAgentID]; ok && agent != nil {
			sessionBackend = agent.Backend
			defaultModel = agent.Model
		}
		if sessionBackend == "" {
			sessionBackend = "codebuddy"
		}
		if defaultModel == "" {
			defaultModel = "glm-5.1"
		}
		var err error
		sessionID, err = service.CreateSession(projectPath, sessionBackend, "新会话", defaultAgentID, defaultModel)
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
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"running": true})
		return
	}

	var req struct {
		Message  string   `json:"message"`
		FilePath string   `json:"filePath"`
		Files    []string `json:"files"`
		AgentID  string   `json:"agentId"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxChatBodySize)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		service.SetSessionRunning(sessionID, false)
		model.WriteErrorf(w, http.StatusBadRequest, "Invalid request")
		return
	}

	// Allow empty message if files are provided
	if req.Message == "" && len(req.Files) == 0 {
		service.SetSessionRunning(sessionID, false)
		model.WriteErrorf(w, http.StatusBadRequest, "Message or files required")
		return
	}

	basePath, _ := filepath.Abs(projectPath)
	var fileDir string
	if req.FilePath == "" {
		fileDir = basePath
	} else {
		fileAbsPath, ok := model.ValidatePath(basePath, req.FilePath)
		if !ok {
			service.SetSessionRunning(sessionID, false)
			model.WriteError(w, model.Forbidden(nil, "Access denied"))
			return
		}
		if _, err := os.Stat(fileAbsPath); err != nil {
			service.SetSessionRunning(sessionID, false)
			model.WriteError(w, model.NotFound(nil, "File not found"))
			return
		}
		fileDir = filepath.Dir(fileAbsPath)
	}

	// Validate file paths are within project and collect absolute paths
	fileAbsPaths := make([]string, 0, len(req.Files))
	for _, fPath := range req.Files {
		fAbsPath, ok := model.ValidatePath(basePath, fPath)
		if !ok {
			service.SetSessionRunning(sessionID, false)
			model.WriteError(w, model.Forbidden(nil, "Access denied"))
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
	if req.FilePath != "" {
		prompt = fmt.Sprintf("[当前文件: %s]\n%s", req.FilePath, req.Message)
	}
	if len(fileAbsPaths) > 0 {
		prompt = fmt.Sprintf("[用户上传了 %d 个文件: %s]\n%s", len(fileAbsPaths), strings.Join(fileAbsPaths, ", "), prompt)
	}

	if _, err := service.AddChatMessage(projectPath, backendName, sessionID, "user", req.Message, req.FilePath, req.Files, false); err != nil {
		service.SetSessionRunning(sessionID, false)
		model.WriteError(w, model.Internal(fmt.Errorf("failed to save message")))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"started": true, "sessionId": sessionID})

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
		agentID = "assistant"
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

		// For OpenCode backend, resolve external session ID when resuming
		effectiveSessionID := sessionID
		resume := service.SessionHasAssistant(sessionID)
		if backendName == "opencode" && resume {
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

		// Use independent context with cancel + timeout to prevent goroutine leaks
		// and support user-initiated cancellation
		ctx, cancel := context.WithCancel(context.Background())
		ctx, timeoutCancel := context.WithTimeout(ctx, 30*time.Minute)
		defer cancel()
		defer timeoutCancel()

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
		var currentText strings.Builder
		var responseMetadata *ai.Metadata

		// Incremental persistence: flush every 1s or every 5 events
		flushTicker := time.NewTicker(1 * time.Second)
		defer flushTicker.Stop()
		eventCount := 0

		serializeBlocks := func() string {
			// Snapshot current blocks including pending text
			snapshotBlocks := make([]model.ContentBlock, len(blocks))
			copy(snapshotBlocks, blocks)
			if currentText.Len() > 0 {
				snapshotBlocks = append(snapshotBlocks, model.ContentBlock{Type: "text", Text: currentText.String()})
			}
			contentMap := map[string]any{"blocks": snapshotBlocks}
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

			accumulateBlock(&blocks, &currentText, event)
			if event.Type == "metadata" && event.Meta != nil {
				responseMetadata = event.Meta
				// Capture OpenCode's external session ID on first response
				if backendName == "opencode" && event.Meta.SessionID != "" {
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
				if len(blocks) > 0 || currentText.Len() > 0 {
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
		// Flush remaining text
		if currentText.Len() > 0 {
			blocks = append(blocks, model.ContentBlock{Type: "text", Text: currentText.String()})
		}

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
	if r.Method != http.MethodPost {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// AIChatStream handles SSE streaming for AI chat responses
func AIChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	_, ok := requireProject(w, r)
	if !ok {
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

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Check if session is running
	if !service.IsSessionRunning(sessionID) {
		fmt.Fprintf(w, "event: error\ndata: {\"error\":\"会话未在运行\"}\n\n")
		if canFlush, ok := w.(http.Flusher); ok {
			canFlush.Flush()
		}
		return
	}

	// Get the stream channel
	streamCh, ok := service.GetSessionStream(sessionID)
	if !ok {
		fmt.Fprintf(w, "event: error\ndata: {\"error\":\"未找到会话流\"}\n\n")
		if canFlush, ok := w.(http.Flusher); ok {
			canFlush.Flush()
		}
		return
	}

	flusher, canFlush := w.(http.Flusher)

	// Periodically check if session is still running.
	// If the cancelled event was dropped (channel full), this is the fallback.
	checkTicker := time.NewTicker(2 * time.Second)
	defer checkTicker.Stop()

	for {
		select {
		case event, ok := <-streamCh:
			if !ok {
				// Channel closed, stream ended
				fmt.Fprintf(w, "event: done\ndata: {}\n\n")
				if canFlush {
					flusher.Flush()
				}
				return
			}

			switch event.Type {
			case "content":
				data, _ := json.Marshal(map[string]string{"content": event.Content})
				fmt.Fprintf(w, "event: content\ndata: %s\n\n", data)
			case "thinking":
				data, _ := json.Marshal(map[string]string{"text": event.Content})
				fmt.Fprintf(w, "event: thinking\ndata: %s\n\n", data)
			case "tool_use":
				if event.Tool != nil {
					var input any
					if event.Tool.Input != "" {
						json.Unmarshal([]byte(event.Tool.Input), &input)
					}
					data, _ := json.Marshal(map[string]any{
						"name":  event.Tool.Name,
						"id":    event.Tool.ID,
						"input": input,
						"done":  event.Tool.Done,
					})
					fmt.Fprintf(w, "event: tool_use\ndata: %s\n\n", data)
				}
			case "metadata":
				data, _ := json.Marshal(event.Meta)
				fmt.Fprintf(w, "event: metadata\ndata: %s\n\n", data)
			case "done":
				fmt.Fprintf(w, "event: done\ndata: {}\n\n")
				if canFlush {
					flusher.Flush()
				}
				return
			case "cancelled":
				data, _ := json.Marshal(map[string]string{"reason": "cancelled"})
				fmt.Fprintf(w, "event: cancelled\ndata: %s\n\n", data)
				if canFlush {
					flusher.Flush()
				}
				return
			case "error":
				data, _ := json.Marshal(map[string]string{"error": event.Error})
				fmt.Fprintf(w, "event: error\ndata: %s\n\n", data)
				if canFlush {
					flusher.Flush()
				}
				return
			case "warning":
				data, _ := json.Marshal(map[string]string{"text": event.Content})
				fmt.Fprintf(w, "event: warning\ndata: %s\n\n", data)
			}

			if canFlush {
				flusher.Flush()
			}

		case <-checkTicker.C:
			// Fallback: if session stopped running but no cancelled event arrived
			// (e.g., channel was full when CancelSession tried to send), synthesize one.
			if !service.IsSessionRunning(sessionID) {
				fmt.Fprintf(w, "event: cancelled\ndata: {\"reason\":\"cancelled\"}\n\n")
				if canFlush {
					flusher.Flush()
				}
				return
			}

		case <-r.Context().Done():
			// Client disconnected — do NOT kill the AI session.
			// The AI goroutine continues running and writing to DB.
			// The frontend will reconnect SSE when it reloads the session.
			slog.Info("sse client disconnected, ai session continues",
				slog.String("session_id", sessionID),
			)
			return
		}
	}
}

// ServeSessions handles GET (list) and POST (create) for chat sessions.
func ServeSessions(w http.ResponseWriter, r *http.Request) {
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Get all sessions across all backends
		sessions, err := service.GetSessions(projectPath, "")
		if err != nil {
			model.WriteError(w, model.Internal(fmt.Errorf("failed to load sessions")))
			return
		}
		// Set running status for each session and convert model name for Claude backend
		for i := range sessions {
			sessions[i].Running = service.IsSessionRunning(sessions[i].ID)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"sessions": sessions})

	case http.MethodPost:
		var req struct {
			Title   string `json:"title"`
			Backend string `json:"backend"`
			AgentID string `json:"agentId"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxChatBodySize)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			model.WriteErrorf(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		// Resolve backend and model from agent config if agent_id is provided
		backend := req.Backend
		agentModel := ""
		agentID := req.AgentID
		if agentID == "" {
			agentID = "assistant"
		}
		if agent, ok := model.Agents[agentID]; ok {
			if agent.Backend != "" {
				backend = agent.Backend
			}
			if agent.Model != "" {
				agentModel = agent.Model
			}
		}
		if backend == "" {
			backend = "codebuddy"
		}
		if agentModel == "" {
			agentModel = "glm-5.1"
		}
		title := req.Title
		if title == "" {
			// Auto-generate title
			existingSessions, err := service.GetSessions(projectPath, backend)
			if err == nil {
				title = fmt.Sprintf("新会话 %d", len(existingSessions)+1)
			} else {
				title = "新会话"
			}
		}
		sessionID, err := service.CreateSession(projectPath, backend, title, agentID, agentModel)
		if err != nil {
			model.WriteError(w, model.Internal(fmt.Errorf("failed to create session")))
			return
		}
		// Set the new session ID in cookie
		setSessionID(w, sessionID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "sessionId": sessionID, "backend": backend, "agentId": agentID})

	default:
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// DeleteSession handles DELETE for a single session.
func DeleteSession(w http.ResponseWriter, r *http.Request) {
	projectPath, ok := requireProject(w, r)
	if !ok {
		return
	}

	if r.Method != http.MethodDelete {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		model.WriteErrorf(w, http.StatusBadRequest, "session_id required")
		return
	}

	// Get backend from query param, required for deleting sessions across backends
	backend := r.URL.Query().Get("backend")
	if backend == "" {
		backend = "codebuddy"
	}

	if err := service.DeleteSession(projectPath, backend, sessionID); err != nil {
		model.WriteError(w, model.Internal(fmt.Errorf("failed to delete session")))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// accumulateBlock processes a single StreamEvent and updates the blocks slice.
// Text events are batched into currentText; thinking events are coalesced;
// tool_use events are deduplicated by ID.
func accumulateBlock(blocks *[]model.ContentBlock, currentText *strings.Builder, event ai.StreamEvent) {
	switch event.Type {
	case "content":
		currentText.WriteString(event.Content)
	case "thinking":
		// Flush pending text first
		if currentText.Len() > 0 {
			*blocks = append(*blocks, model.ContentBlock{Type: "text", Text: currentText.String()})
			currentText.Reset()
		}
		// Coalesce incremental thinking deltas into one block
		if len(*blocks) > 0 && (*blocks)[len(*blocks)-1].Type == "thinking" {
			(*blocks)[len(*blocks)-1].Text += event.Content
		} else {
			*blocks = append(*blocks, model.ContentBlock{Type: "thinking", Text: event.Content})
		}
	case "tool_use":
		if event.Tool != nil {
			// Flush pending text first
			if currentText.Len() > 0 {
				*blocks = append(*blocks, model.ContentBlock{Type: "text", Text: currentText.String()})
				currentText.Reset()
			}
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
		// Flush pending text first
		if currentText.Len() > 0 {
			*blocks = append(*blocks, model.ContentBlock{Type: "text", Text: currentText.String()})
			currentText.Reset()
		}
		*blocks = append(*blocks, model.ContentBlock{Type: "warning", Text: event.Content})
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

// getSessionID retrieves session ID from query param or cookie.
func getSessionID(r *http.Request) string {
	// Try query parameter first
	if sessionID := r.URL.Query().Get("session_id"); sessionID != "" {
		return sessionID
	}
	// Fall back to cookie
	cookie, err := r.Cookie("chat_session_id")
	if err != nil {
		return ""
	}
	return cookie.Value
}

// setSessionID sets session ID in cookie.
func setSessionID(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "chat_session_id",
		Value:    sessionID,
		Path:     "/",
		MaxAge:   86400 * 30, // 30 days
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})
}
