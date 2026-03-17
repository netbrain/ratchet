package handler

import (
	"html/template"
	"log/slog"
	"net/http"
)

// IndexHandler returns a handler that serves the parsed index.html template.
// If the template file is missing or invalid, GET requests return 500 instead
// of panicking.
func IndexHandler(templatePath string) http.Handler {
	tmpl, parseErr := template.ParseFiles(templatePath)
	if parseErr != nil {
		slog.Error("failed to parse index template", "path", templatePath, "error", parseErr)
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

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, nil); err != nil {
			http.Error(w, "failed to render template", http.StatusInternalServerError)
		}
	})
}
