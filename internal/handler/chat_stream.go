package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"clawbench/internal/model"
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
			if !service.IsSessionRunning(sessionID) {
				fmt.Fprintf(w, "event: cancelled\ndata: {\"reason\":\"cancelled\"}\n\n")
				if canFlush {
					flusher.Flush()
				}
				return
			}

		case <-r.Context().Done():
			slog.Info("sse client disconnected, ai session continues",
				slog.String("session_id", sessionID),
			)
			return
		}
	}
}
