// Package assets embeds the compiled Vue frontend into the binary so the
// application ships as a single offline executable.
package assets

import (
	"embed"
	"io/fs"
)

// dist holds the Vite build output. The Makefile populates internal/assets/dist
// before `go build`. The all: prefix also embeds dotfiles (e.g. .gitkeep) so the
// package compiles even before the frontend has been built.
//
//go:embed all:dist
var dist embed.FS

// FS returns the embedded frontend rooted at the dist directory.
func FS() (fs.FS, error) {
	return fs.Sub(dist, "dist")
}

// HasIndex reports whether a built frontend (index.html) is embedded.
func HasIndex() bool {
	sub, err := FS()
	if err != nil {
		return false
	}
	_, err = fs.Stat(sub, "index.html")
	return err == nil
}
