package server

import (
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

// preGzipFileServer serves static files, preferring pre-compressed .gz variants
// when the client accepts gzip encoding.
func preGzipFileServer(fsys fs.FS) http.Handler {
	fallback := http.FileServer(http.FS(fsys))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			fallback.ServeHTTP(w, r)
			return
		}

		// Only try .gz if client accepts gzip.
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			gzPath := path + ".gz"
			if f, err := fsys.Open(gzPath); err == nil {
				f.Close()
				ct := mime.TypeByExtension(filepath.Ext(path))
				if ct == "" {
					ct = "application/octet-stream"
				}
				w.Header().Set("Content-Type", ct)
				w.Header().Set("Content-Encoding", "gzip")
				w.Header().Add("Vary", "Accept-Encoding")
				http.ServeFileFS(w, r, fsys, gzPath)
				return
			}
		}

		fallback.ServeHTTP(w, r)
	})
}
