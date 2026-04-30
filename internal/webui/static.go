package webui

import (
	"embed"
	"io/fs"
)

//go:embed static/*
var staticFiles embed.FS

// staticFS returns the embedded static directory at root.
func staticFS() fs.FS {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		// embed guarantees this never fails at runtime, but be defensive.
		panic(err)
	}
	return sub
}
