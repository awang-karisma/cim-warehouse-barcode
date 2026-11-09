//go:build !embed_frontend

package main

import "io/fs"

var embeddedFS fs.FS
