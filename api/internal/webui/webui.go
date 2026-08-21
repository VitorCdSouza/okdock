package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

func FS() fs.FS {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}

func Placeholder() bool {
	_, err := fs.Stat(FS(), ".placeholder")
	return err == nil
}
