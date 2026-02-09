// Package client provides embedded JavaScript assets for GoliveKit.
package client

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed src/*.js
var assets embed.FS

// Assets returns the embedded filesystem containing JavaScript files.
func Assets() fs.FS {
	fsys, err := fs.Sub(assets, "src")
	if err != nil {
		panic(err)
	}
	return fsys
}

// Handler returns an HTTP handler that serves the embedded assets.
func Handler() http.Handler {
	return http.FileServer(http.FS(Assets()))
}

// MustGetFile returns the contents of an embedded file.
// Panics if the file doesn't exist.
func MustGetFile(name string) []byte {
	data, err := assets.ReadFile("src/" + name)
	if err != nil {
		panic(err)
	}
	return data
}

// GetFile returns the contents of an embedded file.
func GetFile(name string) ([]byte, error) {
	return assets.ReadFile("src/" + name)
}

// FileNames returns the names of all embedded files.
func FileNames() []string {
	entries, err := assets.ReadDir("src")
	if err != nil {
		return nil
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	return names
}
