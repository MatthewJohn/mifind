package api

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web
var webFS embed.FS

// WebFS returns the embedded web UI filesystem.
func WebFS() http.FileSystem {
	return http.FS(webFS)
}

// emptyFS is a minimal filesystem that returns file not found for all operations.
type emptyFS struct{}

func (e *emptyFS) Open(name string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

// spaFileSystem wraps http.FileSystem to handle SPA routing.
// It serves index.html for any path that doesn't match an existing file.
type spaFileSystem struct {
	http.FileSystem
}

// Open opens the named file. If the file doesn't exist, it serves index.html.
func (fs spaFileSystem) Open(name string) (http.File, error) {
	f, err := fs.FileSystem.Open(name)
	if err != nil {
		// File doesn't exist, try index.html for SPA routing
		return fs.FileSystem.Open("/index.html")
	}

	// If it's a directory, try to serve index.html inside it
	stat, _ := f.Stat()
	if stat.IsDir() {
		index, err := fs.FileSystem.Open(name + "/index.html")
		if err == nil {
			return index, nil
		}
	}

	return f, nil
}

// SPAFileSystem returns a handler that serves static files with SPA fallback.
func SPAFileSystem() http.FileSystem {
	return spaFileSystem{WebFS()}
}
