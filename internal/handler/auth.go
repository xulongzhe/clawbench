package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"clawbench/internal/model"
)

// ServeAuthCheck returns 200 if the session cookie is valid, 401 otherwise.
func ServeAuthCheck(w http.ResponseWriter, r *http.Request) {
	if model.SessionToken == "" {
		// No password set, always authenticated
		w.WriteHeader(http.StatusOK)
		return
	}
	token, err := r.Cookie(model.SessionCookie)
	if err != nil || token == nil || token.Value != model.SessionToken {
		model.WriteError(w, model.Unauthorized(nil))
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ServeLogin handles GET (login page) and POST (login attempt).
func ServeLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Serve index.html which mounts the Vue app (LoginView handles auth UI)
		http.ServeFile(w, r, "public/index.html")
		return
	}
	if r.Method == http.MethodPost {
		var body struct{ Password string }
		json.NewDecoder(r.Body).Decode(&body)
		hash := sha256.Sum256([]byte(body.Password + "clawbench-salt"))
		token := hex.EncodeToString(hash[:])
		if model.SessionToken == "" || token == model.SessionToken {
			http.SetCookie(w, &http.Cookie{
				Name:     model.SessionCookie,
				Value:    token,
				Path:     "/",
				HttpOnly: true,
				MaxAge:   int(7 * 24 * 3600),
				SameSite: http.SameSiteStrictMode,
			})
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		} else {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]bool{"ok": false})
		}
		return
	}
	model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
}
