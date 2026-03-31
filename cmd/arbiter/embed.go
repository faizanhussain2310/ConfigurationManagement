//go:build !dev

package main

import (
	"io/fs"
	"log"

	"github.com/faizanhussain/arbiter/web"
)

func getWebFS() fs.FS {
	webFS, err := web.DistFS()
	if err != nil {
		log.Printf("Warning: could not load embedded web assets: %v", err)
		return nil
	}
	return webFS
}
