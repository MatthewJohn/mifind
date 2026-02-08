package api

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web
var webFS embed.FS

// WebFS returns the embedded web UI filesystem.
// It strips the "web" prefix so files are served from root.
func WebFS() http.FileSystem {
	fsys, _ := fs.Sub(webFS, "web")
	return http.FS(fsys)
}

// spaFileSystem wraps http.FileSystem to handle SPA routing.
// It serves index.html for any path that doesn't match an existing file.
type spaFileSystem struct {
	http.FileSystem
}

// Open opens the named file. If the file doesn't exist, it serves index.html.
func (fs spaFileSystem) Open(name string) (http.File, error) {
	// Try opening the requested file first
	f, err := fs.FileSystem.Open(name)
	if err != nil {
		// File doesn't exist, serve index.html for SPA routing
		// Use relative path without leading slash
		return fs.FileSystem.Open("index.html")
	}

	// If it's a directory, try to serve index.html inside it
	stat, _ := f.Stat()
	if stat.IsDir() {
		// Remove trailing slash for consistency
		if name == "/" {
			return fs.FileSystem.Open("index.html")
		}
		index, err := fs.FileSystem.Open(name + "/index.html")
		if err == nil {
			return index, nil
		}
		// If no index.html in subdir, return the directory listing (403)
		return f, nil
	}

	return f, nil
}

// SPAFileSystem returns a handler that serves static files with SPA fallback.
func SPAFileSystem() http.FileSystem {
	return spaFileSystem{WebFS()}
}
