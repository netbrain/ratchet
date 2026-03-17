package handler

import (
	"io/fs"
	"net/http"
)

// StaticHandler returns a handler that serves static files from the given fs.FS.
func StaticHandler(fsys fs.FS) http.Handler {
	return http.StripPrefix("/static/", http.FileServerFS(fsys))
}
