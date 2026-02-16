package main

import (
	"embed"
	"fmt"
	"io/fs"
)

// embeddedWebAssets contains the built frontend assets from web/dist.
// The build pipeline must run `npm --prefix web run build` before `go build`.
//
//go:embed web/dist
var embeddedWebAssets embed.FS

func getEmbeddedWebFS() (fs.FS, error) {
	webFS, err := fs.Sub(embeddedWebAssets, "web/dist")
	if err != nil {
		return nil, fmt.Errorf("failed to access embedded web dist: %w", err)
	}
	if _, err := fs.Stat(webFS, "index.html"); err != nil {
		return nil, fmt.Errorf("embedded web dist is missing index.html: %w", err)
	}
	return webFS, nil
}
