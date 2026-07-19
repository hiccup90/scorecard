package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	root, err := filepath.Abs(s.cfg.StaticDir)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	reqPath := r.URL.Path
	if reqPath == "" || reqPath == "/" {
		http.ServeFile(w, r, filepath.Join(root, "index.html"))
		return
	}

	// Clean and join under root; reject path traversal.
	clean := filepath.Clean("/" + reqPath)
	rel := strings.TrimPrefix(clean, "/")
	full := filepath.Join(root, rel)

	absFull, err := filepath.Abs(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if absFull != root && !strings.HasPrefix(absFull, root+string(os.PathSeparator)) {
		http.NotFound(w, r)
		return
	}

	info, err := os.Stat(absFull)
	if err == nil && !info.IsDir() {
		http.ServeFile(w, r, absFull)
		return
	}

	// SPA fallback
	http.ServeFile(w, r, filepath.Join(root, "index.html"))
}
