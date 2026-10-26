package plotserver

import (
	"embed"
	"io/fs"
)

//go:embed web/*
var embeddedAssets embed.FS

func webAssets() (fs.FS, error) {
	return fs.Sub(embeddedAssets, "web")
}
