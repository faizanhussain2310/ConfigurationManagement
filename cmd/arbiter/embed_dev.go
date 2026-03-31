//go:build dev

package main

import (
	"io/fs"
	"log"

	"github.com/faizanhussain/arbiter/web"
)

func getWebFS() fs.FS {
	webFS, err := web.DistFS()
	if err != nil {
		log.Printf("Dev mode: web/dist not found, dashboard disabled")
		return nil
	}
	return webFS
}
