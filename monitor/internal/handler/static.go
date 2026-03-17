package handler

import (
	"io/fs"
	"net/http"
)

// StaticHandler returns a handler that serves static files from the given
// filesystem. The fsys should contain the files directly (i.e., already
// sub-directed to "static" if using an embedded FS rooted above it).
func StaticHandler(fsys fs.FS) http.Handler {
	return http.StripPrefix("/static/", http.FileServer(http.FS(fsys)))
}
