package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"clawbench/internal/service"
)

// AIChatStream handles SSE streaming for AI chat responses
func AIChatStream(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
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
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "SessionIdRequired")
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Check if session is running
	if !service.IsSessionRunning(sessionID) {
		errMsg := T(r, "SessionNotRunning")
		fmt.Fprintf(w, "event: error\ndata: {\"error\":%q}\n\n", errMsg)
		if canFlush, ok := w.(http.Flusher); ok {
			canFlush.Flush()
		}
		return
	}

	// Get the stream channel
	streamCh, ok := service.GetSessionStream(sessionID)
	if !ok {
		errMsg := T(r, "SessionStreamNotFound")
		fmt.Fprintf(w, "event: error\ndata: {\"error\":%q}\n\n", errMsg)
		if canFlush, ok := w.(http.Flusher); ok {
			canFlush.Flush()
		}
		return
	}

	flusher, canFlush := w.(http.Flusher)

	// Heartbeat: send SSE comment lines to keep the connection alive through
	// reverse proxies and mobile networks during quiet periods (e.g., long-running
	// tool execution). Proxies typically drop idle connections after 30-60s.
	heartbeatTicker := time.NewTicker(15 * time.Second)
	defer heartbeatTicker.Stop()

	// Periodically check if session is still running.
	checkTicker := time.NewTicker(2 * time.Second)
	defer checkTicker.Stop()

	for {
		select {
		case event, ok := <-streamCh:
			if !ok {
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
					if input == nil {
						input = map[string]any{}
					}
					payload := map[string]any{
						"name":  event.Tool.Name,
						"id":    event.Tool.ID,
						"input": input,
						"done":  event.Tool.Done,
					}
					if event.Tool.Output != "" {
						payload["output"] = event.Tool.Output
					}
					if event.Tool.Status != "" {
						payload["status"] = event.Tool.Status
					}
					data, _ := json.Marshal(payload)
					fmt.Fprintf(w, "event: tool_use\ndata: %s\n\n", data)
				}
			case "tool_result":
				if event.Tool != nil {
					payload := map[string]any{
						"id": event.Tool.ID,
					}
					if event.Tool.Output != "" {
						payload["output"] = event.Tool.Output
					}
					if event.Tool.Status != "" {
						payload["status"] = event.Tool.Status
					}
					data, _ := json.Marshal(payload)
					fmt.Fprintf(w, "event: tool_result\ndata: %s\n\n", data)
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
				payload := map[string]string{"error": event.Error}
				if event.Reason != "" {
					payload["reason"] = event.Reason
				}
				data, _ := json.Marshal(payload)
				fmt.Fprintf(w, "event: error\ndata: %s\n\n", data)
				if canFlush {
					flusher.Flush()
				}
				return
			case "warning":
				payload := map[string]string{"text": event.Content}
				if event.Reason != "" {
					payload["reason"] = event.Reason
				}
				data, _ := json.Marshal(payload)
				fmt.Fprintf(w, "event: warning\ndata: %s\n\n", data)
			case "queue_consume":
				if event.QueueEvent != nil {
					data, _ := json.Marshal(map[string]any{
						"text":      event.QueueEvent.Text,
						"filePaths": event.QueueEvent.FilePaths,
						"files":     event.QueueEvent.Files,
					})
					fmt.Fprintf(w, "event: queue_consume\ndata: %s\n\n", data)
				}
			case "queue_update":
				if event.QueueEvent != nil {
					data, _ := json.Marshal(map[string]any{
						"queue": event.QueueEvent.Queue,
					})
					fmt.Fprintf(w, "event: queue_update\ndata: %s\n\n", data)
				}
			case "queue_done":
				fmt.Fprintf(w, "event: queue_done\ndata: {}\n\n")
			case "resume_split":
				// Internal event from AutoResumeBackend: the AI detected ExitPlanMode
				// and will auto-resume. Forward to frontend so it can reset streaming
				// state (clear blocks, prepare for new content after resume).
				fmt.Fprintf(w, "event: resume_split\ndata: {}\n\n")
			}

			if canFlush {
				flusher.Flush()
			}

		case <-heartbeatTicker.C:
			// SSE comment lines (`: ...\n\n`) are ignored by EventSource but keep
			// the TCP connection alive through proxies, load balancers, and
			// mobile networks that drop idle connections.
			fmt.Fprintf(w, ": heartbeat %d\n\n", time.Now().UnixMilli())
			if canFlush {
				flusher.Flush()
			}

		case <-checkTicker.C:
			if !service.IsSessionRunning(sessionID) {
				fmt.Fprintf(w, "event: cancelled\ndata: {\"reason\":\"cancelled\"}\n\n")
				if canFlush {
					flusher.Flush()
				}
				return
			}

		case <-r.Context().Done():
			// SSE client disconnected — do NOT force-cancel the AI session.
			// Disconnections are often transient (Vite HMR, proxy timeout, mobile
			// network switch) and the frontend will reconnect or fall back to polling.
			// Let the AI goroutine finish naturally; it cleans itself up via defers.
			// If no SSE client reconnects, the goroutine still completes and unregisters.
			// Record the disconnect reason so the session finalizer knows the SSE
			// client went away (distinct from an explicit user cancel).
			service.SetCancelReason(sessionID, "disconnect")
			slog.Info("sse client disconnected, ai session continues",
				slog.String("session_id", sessionID),
			)
			return
		}
	}
}
