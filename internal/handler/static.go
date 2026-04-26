package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"clawbench/internal/model"
)

// ServeProjectDialog serves the project dialog HTML template.
func ServeProjectDialog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		model.WriteErrorf(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	tmplPath := filepath.Join("web", "project-dialog.html")
	http.ServeFile(w, r, tmplPath)
}

// ServeIndex serves the main index page and static assets.
func ServeIndex(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Serve index for root — auth is handled by the Vue app itself
	if path == "/" {
		if _, err := os.Stat("public/index.html"); err == nil {
			http.ServeFile(w, r, "public/index.html")
			return
		}
		http.ServeFile(w, r, filepath.Join("web", "index.html"))
		return
	}

	// For other paths (e.g. /index-*.css, /index-*.js), serve from public/
	if _, err := os.Stat("public" + path); err == nil {
		http.ServeFile(w, r, "public"+path)
		return
	}

	// For /css/* paths, also try web/css/
	if strings.HasPrefix(path, "/css/") {
		fallback := filepath.Join("web", path)
		if _, err := os.Stat(fallback); err == nil {
			http.ServeFile(w, r, fallback)
			return
		}
	}

	http.NotFound(w, r)
}
