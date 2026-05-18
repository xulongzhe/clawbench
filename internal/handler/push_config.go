package handler

import (
	"net/http"

	"clawbench/internal/push"
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
