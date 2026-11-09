//go:build embed_frontend

package main

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var frontendFS embed.FS

var embeddedFS fs.FS

func init() {
	sub, err := fs.Sub(frontendFS, "dist")
	if err == nil {
		embeddedFS = sub
	}
}
