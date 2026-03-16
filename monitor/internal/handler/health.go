package handler

import "net/http"

// HealthHandler returns a handler that serves the health check endpoint.
func HealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			methodNotAllowed(w, http.MethodGet+", "+http.MethodHead)
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}
