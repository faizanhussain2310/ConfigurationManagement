//go:build dev

package web

import (
	"io/fs"
	"os"
)

// DistFS returns nil in dev mode. The dev server serves from Vite directly.
func DistFS() (fs.FS, error) {
	if _, err := os.Stat("web/dist"); err == nil {
		return os.DirFS("web/dist"), nil
	}
	return nil, nil
}
