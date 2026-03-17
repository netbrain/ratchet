package handler

import (
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
)

// IndexHandler returns a handler that serves the parsed index.html template
// from the given filesystem. The templateName is the path within the fs
// (e.g. "templates/index.html"). If the template is missing or invalid,
// GET requests return 500 instead of panicking.
func IndexHandler(fsys fs.FS, templateName string) http.Handler {
	tmpl, parseErr := template.ParseFS(fsys, templateName)
	if parseErr != nil {
		slog.Error("failed to parse index template", "name", templateName, "error", parseErr)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}

		if tmpl == nil {
			writeError(w, http.StatusInternalServerError, "template unavailable")
			return
		}

		setSecurityHeaders(w)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, nil); err != nil {
			http.Error(w, "failed to render template", http.StatusInternalServerError)
		}
	})
}
