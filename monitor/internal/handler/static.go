package handler

import "net/http"

// StaticHandler returns a handler that serves files from the given directory.
func StaticHandler(dir string) http.Handler {
	return http.StripPrefix("/static/", http.FileServer(http.Dir(dir)))
}
