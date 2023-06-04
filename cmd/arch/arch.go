package main

import (
	"arch/events"
	"arch/files"
	"arch/files/file_fs"
	"arch/files/mock_fs"
	"arch/lifecycle"
	"arch/model"
	"arch/renderers/tcell"
	"log"
	"os"
)

func main() {
	log.SetFlags(0)

	var paths []string
	if len(os.Args) >= 1 && (os.Args[1] == "-sim" || os.Args[1] == "-sim2") {
		paths = []string{"origin", "copy 1", "copy 2"}
	} else {
		paths = make([]string, len(os.Args)-1)
		var err error
		for i, path := range os.Args[1:] {
			paths[i], err = file_fs.AbsPath(path)
			if err != nil {
				log.Panicf("Failed to scan archives: %#v", err)
			}
		}
	}

	lc := lifecycle.New()
	events := make(events.EventChan, 10)
	renderer, err := tcell.NewRenderer(events)
	if err != nil {
		log.Printf("Failed to open terminal: %#v", err)
		return
	}

	var fs files.FS

	if len(os.Args) >= 1 && os.Args[1] == "-sim" {
		fs = mock_fs.NewFs(events, true)
	} else if len(os.Args) >= 1 && os.Args[1] == "-sim2" {
		fs = mock_fs.NewFs(events, false)
	} else {
		fs = file_fs.NewFs(events, lc)
	}

	model.Run(fs, renderer, events, paths)

	lc.Stop()
	renderer.Stop()
}
