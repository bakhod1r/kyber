// Package web embeds the built React app (web/dist) and serves it as an SPA.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Dist returns the embedded build output.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // the embed pattern guarantees the directory exists
	}
	return sub
}

// Handler serves static files from fsys, falling back to index.html for client-side
// routes. Hashed assets are cached forever; the shell is always revalidated.
func Handler(fsys fs.FS) http.Handler {
	files := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; script-src 'self' https://telegram.org; frame-src https://oauth.telegram.org; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		w.Header().Set("Referrer-Policy", "same-origin")
		index, err := fs.ReadFile(fsys, "index.html")
		if err != nil {
			http.Error(w, "web UI not built: run `npm run build` in web/", http.StatusServiceUnavailable)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" && name != "index.html" {
			if _, err := fs.Stat(fsys, name); err == nil {
				if strings.HasPrefix(name, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
			if strings.HasPrefix(name, "assets/") || path.Ext(name) != "" {
				http.NotFound(w, r)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	})
}
