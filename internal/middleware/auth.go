package middleware

import (
	"crypto/subtle"
	"net"
	"net/http"
	"net/url"

	"clawbench/internal/model"
)

// isLocalhost returns true if the request originates from the local machine.
// CLI subcommands (clawbench task, clawbench rag) always connect from localhost.
func isLocalhost(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}

// Auth wraps a handler with password auth if configured.
// Localhost requests (CLI subcommands) are always allowed.
// Remote requests require a valid "clawbench_session" cookie OR ?token= query parameter.
// The ?token= parameter is for native SSE/HTTP clients that cannot use cookies.
func Auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// No password configured — open access
		if model.SessionToken == "" {
			next.ServeHTTP(w, r)
			return
		}
		// Localhost (CLI subcommands) — always allowed
		if isLocalhost(r) {
			next.ServeHTTP(w, r)
			return
		}
		// Remote — cookie-based auth
		token, err := r.Cookie(model.SessionCookie)
		if err == nil && token != nil && subtle.ConstantTimeCompare([]byte(token.Value), []byte(model.SessionToken)) == 1 {
			next.ServeHTTP(w, r)
			return
		}
		// Remote — ?token= query parameter auth (for native SSE/HTTP clients)
		if qToken := r.URL.Query().Get("token"); qToken != "" &&
			subtle.ConstantTimeCompare([]byte(qToken), []byte(model.SessionToken)) == 1 {
			next.ServeHTTP(w, r)
			return
		}
		model.WriteError(w, model.Unauthorized(nil))
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
