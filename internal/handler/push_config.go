package handler

import (
	"net/http"

	"clawbench/internal/push"
	"clawbench/internal/ws"
)

// pushClientRef holds a reference to the JPush client, set from main.go.
var pushClientRef *push.JPushClient

// SetPushClient stores a reference to the JPush client for handler access.
func SetPushClient(c *push.JPushClient) {
	pushClientRef = c
}

// ServePushConfig returns JPush configuration for the Android app.
// GET /api/push/config
//
// Unauthenticated: the Android native layer calls this before WebView loads
// (no cookies available) to discover the JPush AppKey at runtime.
// Only exposes: jpush_enabled (bool) and jpush_app_key (string) — no secrets.
func ServePushConfig(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	result := map[string]any{
		"jpush_enabled": false,
		"jpush_app_key": "",
	}

	if pushClientRef != nil {
		appKey := pushClientRef.AppKey()
		if pushClientRef.Enabled() && appKey != "" {
			result["jpush_enabled"] = true
			result["jpush_app_key"] = appKey
		}
	}

	writeJSON(w, http.StatusOK, result)
}

// ServePushRegister accepts a JPush Registration ID from the Android app.
// POST /api/push/register
//
// Authenticated: called after login (WebView has session cookie).
// The Registration ID is a login-level lifecycle event, not per-WS-connection.
// It persists in the WS Manager so that BroadcastEvent can send JPush
// notifications when the client's WebSocket is disconnected.
func ServePushRegister(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var body struct {
		RegistrationID string `json:"registration_id"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	if body.RegistrationID == "" {
		writeLocalizedErrorf(w, r, http.StatusBadRequest, "InvalidRequestBody")
		return
	}

	mgr := ws.GetManager()
	if mgr != nil {
		mgr.RegisterPushID(body.RegistrationID)
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
