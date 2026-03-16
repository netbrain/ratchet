package handler

import (
	"html/template"
	"net/http"
)

// IndexHandler returns a handler that serves the parsed index.html template.
func IndexHandler(templatePath string) http.Handler {
	tmpl := template.Must(template.ParseFiles(templatePath))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, nil); err != nil {
			http.Error(w, "failed to render template", http.StatusInternalServerError)
		}
	})
}
