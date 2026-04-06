package staticspa

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Handler serves multiple SPA bundles from a shared static root.
// Root routes fall back to /index.html, while /apps/<id> routes fall back
// to /apps/<id>/index.html when no concrete file exists.
type Handler struct {
	root string
}

func New(root string) http.Handler {
	return &Handler{root: root}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	requestPath := path.Clean("/" + r.URL.Path)
	if filePath, ok := h.resolveFile(requestPath); ok {
		http.ServeFile(w, r, filePath)
		return
	}

	if hasExtension(requestPath) {
		http.NotFound(w, r)
		return
	}

	if indexPath, ok := h.resolveSPAIndex(requestPath); ok {
		http.ServeFile(w, r, indexPath)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) resolveFile(requestPath string) (string, bool) {
	target := filepath.Join(h.root, filepath.FromSlash(strings.TrimPrefix(requestPath, "/")))
	info, err := os.Stat(target)
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		indexPath := filepath.Join(target, "index.html")
		if fileExists(indexPath) {
			return indexPath, true
		}
		return "", false
	}
	return target, true
}

func (h *Handler) resolveSPAIndex(requestPath string) (string, bool) {
	segments := strings.Split(strings.Trim(requestPath, "/"), "/")
	if len(segments) >= 2 && segments[0] == "apps" && segments[1] != "" {
		indexPath := filepath.Join(h.root, "apps", segments[1], "index.html")
		if fileExists(indexPath) {
			return indexPath, true
		}
		return "", false
	}

	indexPath := filepath.Join(h.root, "index.html")
	if fileExists(indexPath) {
		return indexPath, true
	}
	return "", false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func hasExtension(requestPath string) bool {
	return path.Ext(requestPath) != ""
}
