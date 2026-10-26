package static

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

const (
	assetsPrefix    = "assets/"
	cacheImmutable  = "public, max-age=31536000, immutable"
	cacheRevalidate = "no-cache"
)

type spa struct {
	root  fs.FS
	files http.Handler
}

// NewSPA serves the built frontend from root. Unknown extension-less paths get
// index.html so client-side routes work on refresh; unknown files get a 404.
func NewSPA(root fs.FS) http.Handler {
	return &spa{root: root, files: http.FileServerFS(root)}
}

func (s *spa) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	switch {
	case name == "":
		s.serveIndex(w, r)
	case isFile(s.root, name):
		w.Header().Set("Cache-Control", cacheControl(name))
		s.files.ServeHTTP(w, r)
	case path.Ext(name) != "":
		http.NotFound(w, r)
	default:
		s.serveIndex(w, r)
	}
}

func (s *spa) serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", cacheRevalidate)
	// The file server serves index.html for the directory root.
	index := r.Clone(r.Context())
	index.URL.Path = "/"
	s.files.ServeHTTP(w, index)
}

func isFile(root fs.FS, name string) bool {
	info, err := fs.Stat(root, name)
	return err == nil && !info.IsDir()
}

// Vite fingerprints everything under assets/, so those files never change.
func cacheControl(name string) string {
	if strings.HasPrefix(name, assetsPrefix) {
		return cacheImmutable
	}
	return cacheRevalidate
}
