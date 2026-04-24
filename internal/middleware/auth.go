package middleware

import (
	"net/http"
	"net/url"

	"clawbench/internal/model"
)

// Auth wraps a handler with password auth if configured.
func Auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if model.SessionToken == "" {
			next.ServeHTTP(w, r)
			return
		}
		token, err := r.Cookie(model.SessionCookie)
		if err != nil || token == nil || token.Value != model.SessionToken {
			model.WriteError(w, model.Unauthorized(nil))
			return
		}
		next.ServeHTTP(w, r)
	}
}

// GetProjectFromCookie extracts the current project path from cookie.
func GetProjectFromCookie(r *http.Request) string {
	cookie, err := r.Cookie("clawbench_project")
	if err != nil || cookie == nil || cookie.Value == "" {
		return ""
	}
	decoded, decErr := url.QueryUnescape(cookie.Value)
	if decErr != nil {
		return cookie.Value
	}
	return decoded
}
