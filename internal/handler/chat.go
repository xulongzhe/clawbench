package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
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
				sessionID, err = service.CreateSession(projectPath, sessionBackend2, "新会话", agentID, defaultModel, "default")
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
		// Check session count limit before auto-creating (0 = unlimited)
		if model.SessionMaxCount > 0 {
			if count, cerr := service.GetSessionCount(projectPath); cerr == nil && count >= model.SessionMaxCount {
				model.WriteErrorf(w, http.StatusConflict, fmt.Sprintf("已达会话数量上限（%d），请先删除旧会话", model.SessionMaxCount))
				return
			}
		}
		agentID2 := model.GetDefaultAgentID()
		sessionBackend2, defaultModel2, _, _, ok := resolveAgentConfig(agentID2)
		if !ok {
			model.WriteErrorf(w, http.StatusServiceUnavailable, "no agents available")
			return
		}
		var err error
		sessionID, err = service.CreateSession(projectPath, sessionBackend2, "新会话", agentID2, defaultModel2, "default")
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

	// Decode request body BEFORE the running check so we can enqueue when busy
	var req struct {
		Message   string   `json:"message"`
		FilePath  string   `json:"filePath"`  // legacy: single file path
		FilePaths []string `json:"filePaths"` // new: multiple file paths
		Files     []string `json:"files"`
		AgentID   string   `json:"agentId"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxChatBodySize)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		model.WriteErrorf(w, http.StatusBadRequest, "Invalid request")
		return
	}

	// Allow empty message if files are provided
	if req.Message == "" && len(req.Files) == 0 && len(req.FilePaths) == 0 {
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
			return
		}
		if _, err := os.Stat(firstAbsPath); err != nil {
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
			return
		}
		if _, err := os.Stat(fAbsPath); err != nil {
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
			return
		}
		if _, err := os.Stat(fAbsPath); err != nil {
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

	// Resolve agent config early (needed for both enqueue and execution paths)
	effectiveAgentID := req.AgentID
	if effectiveAgentID == "" {
		effectiveAgentID = model.GetDefaultAgentID()
	}

	// Prevent concurrent sessions for the same session ID
	if !service.TrySetSessionRunning(sessionID) {
		// Session already running — enqueue the message
		qMsg := model.QueuedMessage{
			Text:      req.Message,
			FilePath:  legacyFilePath,
			FilePaths: allFilePaths,
			Files:     allFiles,
			CreatedAt: time.Now().Format(time.RFC3339),
		}
		queueState := service.EnqueueMessage(sessionID, qMsg)

		// Persist user message to DB immediately
		service.AddChatMessage(projectPath, backendName, sessionID, "user", req.Message, legacyFilePath, allFiles, false)

		// Notify the running goroutine via SSE
		service.SendSessionEvent(sessionID, ai.StreamEvent{
			Type:       "queue_update",
			QueueEvent: &ai.QueueEventData{Queue: queueState},
		})

		writeJSON(w, http.StatusOK, map[string]any{
			"running": true,
			"queued":  true,
			"queue":   queueState,
		})
		return
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
				service.SendSessionEvent(sessionID, ai.StreamEvent{Type: "error", Error: "AI internal error, please retry", Reason: ai.ReasonPanic})
				service.UnregisterSessionStream(sessionID)
				// Persist error to database
				errMsg := "AI internal error, please retry"
				errContent, _ := json.Marshal(map[string]any{"blocks": []any{map[string]string{"type": "error", "text": errMsg, "reason": ai.ReasonPanic}}})
				service.FinalizeStreamingMessage(projectPath, backendName, sessionID, string(errContent))
			}
		}()
		slog.Info("ai goroutine started", slog.String("project", projectPath))
		defer service.SetSessionRunning(sessionID, false)
		defer service.UnregisterSessionStream(sessionID)

		// Use independent context with cancel to prevent goroutine leaks
		// and support user-initiated cancellation (no timeout - let AI run indefinitely)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		service.RegisterSessionCancel(sessionID, cancel)
		defer service.UnregisterSessionCancel(sessionID)

		// Build the first chat request
		firstChatReq := buildChatRequest(prompt, sessionID, backendName, effectiveAgentID, fileDir)

		// Execute first message
		result := executeStreamRun(ctx, streamCh, projectPath, sessionID, backendName, effectiveAgentID, firstChatReq, fileDir)

		// Drain loop: keep executing queued messages after normal completion
		for {
			if result.cancelReason == "user" || result.cancelReason == "disconnect" {
				service.ClearQueue(sessionID)
				sendFinalEvent(streamCh, ai.StreamEvent{Type: "cancelled"})
				return
			}
			if result.err != "" {
				sendFinalEvent(streamCh, ai.StreamEvent{Type: "error", Error: result.err})
				return
			}
			if result.empty {
				sendFinalEvent(streamCh, ai.StreamEvent{Type: "error", Error: "AI returned no content", Reason: ai.ReasonEmpty})
				return
			}
			if result.cancelReason != "" {
				// Other cancel reasons
				sendFinalEvent(streamCh, ai.StreamEvent{Type: "cancelled"})
				return
			}

			// Normal completion — check queue for next message
			qMsg, ok := service.DequeueMessage(sessionID)
			if !ok {
				// Brief re-check for enqueue-during-exit race
				time.Sleep(50 * time.Millisecond)
				qMsg, ok = service.DequeueMessage(sessionID)
			}
			if !ok {
				// Queue empty — truly done
				sendFinalEvent(streamCh, ai.StreamEvent{Type: "done"})
				return
			}

			// Queue has next message — send queue_consume + queue_update, persist, execute
			slog.Info("draining queued message", slog.String("session", sessionID), slog.String("text", qMsg.Text))

			// Notify frontend: a queued message is about to execute
			sendEvent(ctx, streamCh, ai.StreamEvent{
				Type:       "queue_consume",
				QueueEvent: &ai.QueueEventData{Text: qMsg.Text, FilePath: qMsg.FilePath, FilePaths: qMsg.FilePaths, Files: qMsg.Files},
			})

			// Persist user message to DB
			service.AddChatMessage(projectPath, backendName, sessionID, "user", qMsg.Text, qMsg.FilePath, qMsg.Files, false)

			// Send updated queue state
			remainingQueue := service.GetQueue(sessionID)
			sendEvent(ctx, streamCh, ai.StreamEvent{
				Type:       "queue_update",
				QueueEvent: &ai.QueueEventData{Queue: remainingQueue},
			})

			// Build chat request from queued message and execute
			nextChatReq := buildChatRequestFromQueue(qMsg, sessionID, projectPath, backendName, effectiveAgentID, fileDir)
			result = executeStreamRun(ctx, streamCh, projectPath, sessionID, backendName, effectiveAgentID, nextChatReq, fileDir)
			// Loop continues
		}
	}()
}

// streamRunResult captures the outcome of a single AI stream execution.
type streamRunResult struct {
	cancelReason string // "", "user", "disconnect"
	err          string // error message if execution failed
	empty        bool   // true if AI returned no content
}

// executeStreamRun runs one AI backend execution from start to finish.
// It handles event accumulation, incremental DB persistence, resume_split,
// and finalizes the streaming message in the DB.
// It does NOT send a terminal SSE event — the caller decides what to send.
func executeStreamRun(
	ctx context.Context,
	streamCh chan<- ai.StreamEvent,
	projectPath, sessionID, backendName, agentID string,
	chatReq ai.ChatRequest,
	fileDir string,
) streamRunResult {
	backend, err := ai.NewBackend(backendName)
	if err != nil {
		slog.Error("failed to create backend", slog.String("backend", backendName), slog.String("err", err.Error()))
		errMsg := fmt.Sprintf("创建 AI Backend 失败: %v", err)
		if !sendEvent(ctx, streamCh, ai.StreamEvent{Type: "error", Error: errMsg}) {
			return streamRunResult{err: errMsg}
		}
		_, _ = service.AddChatMessage(projectPath, backendName, sessionID, "assistant", errMsg, "", nil, false)
		return streamRunResult{err: errMsg}
	}

	eventCh, err := backend.ExecuteStream(ctx, chatReq)
	if err != nil {
		slog.Error("failed to start stream", slog.String("err", err.Error()))
		errMsg := fmt.Sprintf("启动流式输出失败: %v", err)
		if !sendEvent(ctx, streamCh, ai.StreamEvent{Type: "error", Error: errMsg}) {
			return streamRunResult{err: errMsg}
		}
		_, _ = service.AddChatMessage(projectPath, backendName, sessionID, "assistant", errMsg, "", nil, false)
		return streamRunResult{err: errMsg}
	}

	// Create streaming placeholder message in DB
	emptyContent, _ := json.Marshal(map[string]any{"blocks": []any{}})
	_, _ = service.AddChatMessage(projectPath, backendName, sessionID, "assistant", string(emptyContent), "", nil, true)

	var blocks []model.ContentBlock
	var responseMetadata *ai.Metadata
	var rawOutput string // collected from raw_output event for debugging

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
				// Stream ended — finalize below
				return finalizeStreamRun(ctx, streamCh, projectPath, backendName, sessionID, agentID, chatReq, blocks, responseMetadata, rawOutput, eventCh)
			}
			// Don't forward "done" here — finalize below
			if event.Type == "done" {
				return finalizeStreamRun(ctx, streamCh, projectPath, backendName, sessionID, agentID, chatReq, blocks, responseMetadata, rawOutput, eventCh)
			}
			// Capture raw output for debugging (not forwarded to SSE)
			if event.Type == "raw_output" {
				rawOutput = event.RawOutput
				continue
			}
			// Early capture of external session ID (OpenCode/Codex).
			// Persist immediately so that if the stream is cancelled before
			// step_finish/turn.completed, the ID is already saved for resumption.
			if event.Type == "session_capture" {
				if (backendName == "opencode" || backendName == "codex") && event.Content != "" {
					existingExtID := service.GetExternalSessionID(sessionID)
					if existingExtID == "" {
						if err := service.UpdateExternalSessionID(sessionID, event.Content); err != nil {
							slog.Error("failed to save external session ID (early capture)",
								slog.String("session", sessionID),
								slog.String("external_id", event.Content),
								slog.String("err", err.Error()),
							)
						} else {
							slog.Info("early-captured external session ID",
								slog.String("session", sessionID),
								slog.String("external_id", event.Content))
						}
					}
				}
				continue
			}
			// Forward to SSE channel
			if !sendEvent(ctx, streamCh, event) {
				return finalizeStreamRun(ctx, streamCh, projectPath, backendName, sessionID, agentID, chatReq, blocks, responseMetadata, rawOutput, eventCh)
			}

			ai.AccumulateBlock(&blocks, event)

			// Handle resume_split: the AI adapter layer detected ExitPlanMode and
			// will auto-resume. Finalize current DB message and start a new one.
			if event.Type == "resume_split" {
				slog.Info("resume_split received, finalizing current message and starting new one",
					slog.String("session", sessionID))

				// Finalize current streaming message
				if err := service.FinalizeStreamingMessage(projectPath, backendName, sessionID, serializeBlocks()); err != nil {
					slog.Error("failed to finalize pre-resume message",
						slog.String("session", sessionID),
						slog.String("err", err.Error()))
				}

				// Save raw output if captured so far
				if rawOutput != "" {
					if msgID := service.GetStreamingMessageID(sessionID); msgID > 0 {
						if err := service.SaveRawResponse(sessionID, backendName, msgID, rawOutput); err != nil {
							slog.Error("failed to save raw response",
								slog.String("session", sessionID),
								slog.String("err", err.Error()))
						}
					}
					rawOutput = ""
				}

				// Reset blocks and metadata for the resumed stream
				blocks = nil
				responseMetadata = nil
				eventCount = 0

				// Create new streaming assistant placeholder
				emptyContent, _ = json.Marshal(map[string]any{"blocks": []any{}})
				if _, err := service.AddChatMessage(projectPath, backendName, sessionID, "assistant", string(emptyContent), "", nil, true); err != nil {
					slog.Error("failed to create resume streaming message",
						slog.String("session", sessionID),
						slog.String("err", err.Error()))
					return streamRunResult{err: "failed to create resume streaming message"}
				}
				continue
			}

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
}

// finalizeStreamRun handles the finalize phase of a stream run: schedule-proposal detection,
// DB finalization, raw output saving, and determining the result.
// It does NOT send a terminal SSE event.
func finalizeStreamRun(
	ctx context.Context,
	streamCh chan<- ai.StreamEvent,
	projectPath, backendName, sessionID, agentID string,
	chatReq ai.ChatRequest,
	blocks []model.ContentBlock,
	responseMetadata *ai.Metadata,
	rawOutput string,
	eventCh <-chan ai.StreamEvent,
) streamRunResult {
	// Detect <ask-question> in the fully accumulated text blocks and convert to tool_use blocks.
	// This enables all backends (not just Claude/Codebuddy) to produce interactive question cards.
	if stringsContainsAnyBlock(blocks, "<ask-question") {
		slog.Info("detected ask-question tag(s) in accumulated text blocks",
			slog.String("session", sessionID),
		)
		blocks = convertAskQuestionBlocks(blocks)
	}

	// Detect <schedule-proposal> in the fully accumulated text blocks.
	if !chatReq.ScheduledExecution {
		for i := range blocks {
			if blocks[i].Type == "text" && strings.Contains(blocks[i].Text, "<schedule-proposal") {
				slog.Info("detected schedule-proposal tag in accumulated text block",
					slog.String("session", sessionID),
				)
				if taskID := detectAndCreateScheduleProposal(blocks[i].Text, projectPath, sessionID, agentID); taskID != "" {
					blocks[i].Text = injectTaskIDIntoProposal(blocks[i].Text, taskID)
				}
			}
		}
	}

	// Determine cancellation reason
	cancelReason := service.GetAndClearCancelReason(sessionID)

	// Serialize blocks + metadata as JSON for database storage
	var content string
	if len(blocks) == 0 {
		// Auto-infer reason for empty response
		var errMsg string
		var reason string
		switch {
		case cancelReason == "user":
			errMsg, reason = "User cancelled", ai.ReasonUserCancel
		case cancelReason == "disconnect":
			errMsg, reason = "Connection lost, AI response interrupted", ai.ReasonDisconnect
		case ctx.Err() == context.Canceled:
			errMsg, reason = "AI response cancelled", ai.ReasonContextCancel
		case ctx.Err() == context.DeadlineExceeded:
			errMsg, reason = "AI response timed out (30 min)", ai.ReasonTimeout
		default:
			errMsg, reason = "AI returned no content", ai.ReasonEmpty
		}
		blocks = append(blocks, model.ContentBlock{Type: "warning", Text: errMsg, Reason: reason})
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
			blocks = append(blocks, model.ContentBlock{Type: "warning", Text: "Connection lost, AI response interrupted", Reason: ai.ReasonDisconnect})
			contentMap["cancelled"] = true
		} else if cancelReason == "user" {
			contentMap["cancelled"] = true
		} else if ctx.Err() == context.Canceled {
			contentMap["cancelled"] = true
		} else if ctx.Err() == context.DeadlineExceeded {
			blocks = append(blocks, model.ContentBlock{Type: "warning", Text: "AI response timed out (30 min)", Reason: ai.ReasonTimeout})
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

	// Drain any remaining events from channel
	for {
		select {
		case event, ok := <-eventCh:
			if !ok {
				goto saveRaw
			}
			if event.Type == "raw_output" && rawOutput == "" {
				rawOutput = event.RawOutput
			}
		default:
			goto saveRaw
		}
	}

saveRaw:
	// Save raw AI backend output for debugging/analysis
	if rawOutput != "" {
		if msgID := service.GetStreamingMessageID(sessionID); msgID > 0 {
			if err := service.SaveRawResponse(sessionID, backendName, msgID, rawOutput); err != nil {
				slog.Error("failed to save raw response",
					slog.String("session", sessionID),
					slog.String("err", err.Error()),
				)
			}
		}
	}

	// Build result — do NOT send terminal SSE event here
	result := streamRunResult{}

	if cancelReason == "user" || cancelReason == "disconnect" {
		result.cancelReason = cancelReason
	} else if ctx.Err() == context.Canceled {
		result.cancelReason = "cancel"
	} else if ctx.Err() == context.DeadlineExceeded {
		result.err = "AI response timed out (30 min)"
	} else if len(blocks) == 0 {
		result.empty = true
	}

	slog.Info("ai stream run done",
		slog.String("session", sessionID),
		slog.Int("blocks", len(blocks)),
		slog.String("cancel_reason", cancelReason),
	)

	return result
}

// buildChatRequest constructs an ai.ChatRequest from the given parameters.
func buildChatRequest(prompt, sessionID, backendName, agentID, fileDir string) ai.ChatRequest {
	systemPrompt := ""
	agentModel := ""
	agentCommand := ""

	if agentID == "" {
		agentID = model.GetDefaultAgentID()
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
		} else {
			// No external session ID available — don't pass the invalid ClawBench UUID
			// to OpenCode/Codex. They don't recognize it and would silently fail
			// (stdout empty, exit 0), resulting in "AI returned no content".
			// Let them start a fresh session instead.
			effectiveSessionID = ""
		}
	}

	return ai.ChatRequest{
		Prompt:       prompt,
		SessionID:    effectiveSessionID,
		WorkDir:      fileDir,
		SystemPrompt: systemPrompt,
		Model:        agentModel,
		Command:      agentCommand,
		AgentID:      agentID,
		Resume:       resume,
	}
}

// buildChatRequestFromQueue constructs an ai.ChatRequest from a queued message.
func buildChatRequestFromQueue(qMsg model.QueuedMessage, sessionID, projectPath, backendName, agentID, fileDir string) ai.ChatRequest {
	prompt := qMsg.Text
	if len(qMsg.FilePaths) > 0 {
		prompt = fmt.Sprintf("[当前文件: %s]\n%s", strings.Join(qMsg.FilePaths, ", "), qMsg.Text)
	}
	if len(qMsg.Files) > 0 {
		prompt = fmt.Sprintf("[用户上传了 %d 个文件: %s]\n%s", len(qMsg.Files), strings.Join(qMsg.Files, ", "), prompt)
	}

	return buildChatRequest(prompt, sessionID, backendName, agentID, fileDir)
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// detectAndCreateScheduleProposal detects <schedule-proposal> tags in text and automatically creates scheduled tasks.
// It extracts the JSON content from the tag, validates it, and creates the task.
// Errors are logged but don't interrupt the stream - the proposal tag is preserved for frontend display.
// Returns the created task ID on success, or empty string on failure.
func detectAndCreateScheduleProposal(text, projectPath, sessionID, agentID string) string {
	// Extract the schedule-proposal tag content
	re := regexp.MustCompile(`<schedule-proposal\b[^>]*>([\s\S]*?)</schedule-proposal>`)
	matches := re.FindStringSubmatch(text)
	if len(matches) < 2 {
		return ""
	}

	jsonStr := strings.TrimSpace(matches[1])

	// Parse the JSON
	var proposal struct {
		Name       string `json:"name"`
		CronExpr   string `json:"cron_expr"`
		AgentID    string `json:"agent_id"`
		Prompt     string `json:"prompt"`
		RepeatMode string `json:"repeat_mode"`
		MaxRuns    int    `json:"max_runs"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &proposal); err != nil {
		slog.Error("failed to parse schedule proposal JSON",
			slog.String("session", sessionID),
			slog.String("error", err.Error()),
		)
		return ""
	}

	// Validate required fields
	if proposal.Name == "" || proposal.CronExpr == "" || proposal.AgentID == "" || proposal.Prompt == "" {
		slog.Error("schedule proposal missing required fields",
			slog.String("session", sessionID),
			slog.String("name", proposal.Name),
			slog.String("cron_expr", proposal.CronExpr),
			slog.String("agent_id", proposal.AgentID),
			slog.String("prompt", proposal.Prompt),
		)
		return ""
	}

	// Use the agent from the proposal if specified, otherwise use the session's agent
	effectiveAgentID := proposal.AgentID
	if effectiveAgentID == "" {
		effectiveAgentID = agentID
	}

	// Set defaults
	if proposal.RepeatMode == "" {
		proposal.RepeatMode = "unlimited"
	}

	// Create the task
	task := &model.ScheduledTask{
		ProjectPath: projectPath,
		Name:        proposal.Name,
		CronExpr:    proposal.CronExpr,
		AgentID:     effectiveAgentID,
		Prompt:      proposal.Prompt,
		RepeatMode:  proposal.RepeatMode,
		MaxRuns:     proposal.MaxRuns,
		SessionID:   sessionID,
	}

	if err := service.GlobalScheduler.AddTask(task); err != nil {
		slog.Error("failed to create scheduled task from proposal",
			slog.String("session", sessionID),
			slog.String("task_name", proposal.Name),
			slog.String("error", err.Error()),
		)
		return ""
	}

	slog.Info("automatically created scheduled task from proposal",
		slog.String("session", sessionID),
		slog.String("task_id", task.ID),
		slog.String("task_name", proposal.Name),
		slog.String("cron_expr", proposal.CronExpr),
	)
	return task.ID
}

// injectTaskIDIntoProposal adds a "task_id" field to the JSON inside a <schedule-proposal> tag.
// This allows the frontend to link the proposal card to the created task for editing.
func injectTaskIDIntoProposal(text, taskID string) string {
	re := regexp.MustCompile(`<schedule-proposal\b[^>]*>([\s\S]*?)</schedule-proposal>`)
	matches := re.FindStringSubmatch(text)
	if len(matches) < 2 {
		return text
	}

	var proposal map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(matches[1])), &proposal); err != nil {
		return text
	}

	proposal["task_id"] = taskID
	updatedJSON, err := json.Marshal(proposal)
	if err != nil {
		return text
	}

	return re.ReplaceAllString(text, "<schedule-proposal>"+string(updatedJSON)+"</schedule-proposal>")
}

// stringsContainsAnyBlock checks if any text ContentBlock contains the given substring.
func stringsContainsAnyBlock(blocks []model.ContentBlock, substr string) bool {
	for _, b := range blocks {
		if b.Type == "text" && strings.Contains(b.Text, substr) {
			return true
		}
	}
	return false
}

// convertAskQuestionBlocks detects <ask-question> tags in text ContentBlocks,
// parses the JSON content, and converts them into tool_use ContentBlocks with
// name="AskUserQuestion". Tags are stripped from text; if no text remains the
// block is replaced entirely, otherwise a new tool_use block is appended.
// Returns the updated blocks slice.
func convertAskQuestionBlocks(blocks []model.ContentBlock) []model.ContentBlock {
	re := regexp.MustCompile(`<ask-question\b[^>]*>([\s\S]*?)</ask-question>`)

	// First pass: collect all conversions needed
	type conversion struct {
		index     int
		input     map[string]any
		cleanText string
	}
	var conversions []conversion

	for i, block := range blocks {
		if block.Type != "text" {
			continue
		}
		if !strings.Contains(block.Text, "<ask-question") {
			continue
		}
		matches := re.FindStringSubmatch(block.Text)
		if len(matches) < 2 {
			continue
		}

		var input map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(matches[1])), &input); err != nil {
			slog.Error("failed to parse ask-question JSON", slog.String("error", err.Error()))
			continue
		}

		questions, ok := input["questions"]
		if !ok {
			slog.Error("ask-question missing 'questions' field")
			continue
		}
		questionsArr, ok := questions.([]any)
		if !ok || len(questionsArr) == 0 {
			slog.Error("ask-question 'questions' must be a non-empty array")
			continue
		}

		cleanText := strings.TrimSpace(re.ReplaceAllString(block.Text, ""))
		conversions = append(conversions, conversion{index: i, input: input, cleanText: cleanText})
	}

	// Apply conversions in reverse order so index shifts don't affect earlier entries
	for i := len(conversions) - 1; i >= 0; i-- {
		c := conversions[i]
		toolBlock := model.ContentBlock{
			Type:  "tool_use",
			Name:  "AskUserQuestion",
			ID:    fmt.Sprintf("ask-%d", time.Now().UnixNano()%1000000),
			Input: c.input,
			Done:  true,
		}

		if c.cleanText == "" {
			// No remaining text — replace the text block with the tool_use block
			blocks[c.index] = toolBlock
		} else {
			// Has remaining text — strip the tag and insert tool_use block after
			blocks[c.index].Text = c.cleanText
			// Insert tool_use block after the text block
			insertAt := c.index + 1
			blocks = append(blocks[:insertAt], append([]model.ContentBlock{toolBlock}, blocks[insertAt:]...)...)
		}
	}

	return blocks
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
		toolID := ""
		if event.Tool != nil {
			toolID = event.Tool.ID
		}
		slog.Warn("SSE event dropped — channel full",
			slog.String("type", event.Type),
			slog.String("tool_id", toolID),
		)
		return true
	}
}

// sendFinalEvent sends a terminal event (done/cancelled/error) to the stream channel
// without checking context cancellation. This ensures the SSE client always receives
// the terminal event even after the CLI context has been cancelled (e.g. ExitPlanMode).
func sendFinalEvent(ch chan<- ai.StreamEvent, event ai.StreamEvent) {
	select {
	case ch <- event:
	default:
		slog.Warn("SSE terminal event dropped — channel full",
			slog.String("type", event.Type),
		)
	}
}
