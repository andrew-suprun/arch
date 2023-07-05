package controller

import (
	m "arch/model"
	w "arch/widgets"
	"log"
	"strings"
)

type screenBuilder struct {
	*controller
	duplicateCounts map[m.Hash]int
	copyHashName    map[m.Hash]m.Name
	absentHashes    map[m.Hash]struct{}
	originHashed    bool
}

func (c *controller) buildScreen() *w.Screen {
	builder := &screenBuilder{
		controller:      c,
		duplicateCounts: map[m.Hash]int{},
		copyHashName:    map[m.Hash]m.Name{},
		absentHashes:    map[m.Hash]struct{}{},
	}
	builder.buildEntries()

	folder := c.folders[c.currentPath]
	screen := &w.Screen{
		CurrentPath:    c.currentPath,
		Progress:       c.progress(),
		SelectedId:     folder.selectedId,
		OffsetIdx:      folder.offsetIdx,
		SortColumn:     folder.sortColumn,
		SortAscending:  folder.sortAscending,
		PendingFiles:   c.pendingFiles,
		DuplicateFiles: c.duplicateFiles,
		AbsentFiles:    c.absentFiles,
	}

	screen.Entries = make([]w.File, len(c.entries))
	copy(screen.Entries, c.entries)
	return screen
}

func (b *screenBuilder) buildEntries() {
	log.Printf("\n\n### buildEntries: --------")
	b.controller.pendingFiles, b.controller.duplicateFiles, b.controller.absentFiles = 0, 0, 0
	b.controller.entries = b.controller.entries[:0]

	b.duplicates(b.controller.archives[b.controller.origin])

	b.handleOrigin(b.controller.archives[b.controller.origin])

	for _, root := range b.controller.roots[1:] {
		b.handleCopy(b.controller.archives[root])
	}

	b.controller.sort()
}

func (b *screenBuilder) duplicates(a *archive) {
	b.originHashed = true
	for _, file := range a.infoByName {
		if file.Hash != "" {
			b.duplicateCounts[file.Hash]++
		} else {
			b.originHashed = false
		}
	}
	for _, count := range b.duplicateCounts {
		if count > 1 {
			b.controller.duplicateFiles++
		}
	}
}

func (b *screenBuilder) handleOrigin(archive *archive) {
	log.Printf("### handleOrigin ---- ")
	for _, file := range archive.infoByName {
		if file.Path == b.controller.currentPath {
			entry := w.File{
				FileMeta: file.FileMeta,
				FileKind: w.FileRegular,
				Hash:     file.Hash,
			}
			if b.duplicateCounts[file.Hash] > 1 {
				entry.Status = w.Duplicate
			}
			b.controller.entries = append(b.controller.entries, entry)
			log.Printf("### handleOrigin.1: entry %v", entry)
		} else if strings.HasPrefix(file.Path.String(), b.controller.currentPath.String()) {
			relPath := file.Path
			if len(b.controller.currentPath) > 0 {
				relPath = file.Path[len(b.controller.currentPath)+1:]
			}
			name := m.Name(strings.SplitN(relPath.String(), "/", 2)[0])

			i, found := m.Find(b.controller.entries, func(entry w.File) bool { return name == entry.Name })
			if found {
				b.controller.entries[i].Size += file.Size
				if b.controller.entries[i].ModTime.Before(file.ModTime) {
					b.controller.entries[i].ModTime = file.ModTime
				}
				if b.duplicateCounts[file.Hash] > 1 {
					b.controller.entries[i].Status = w.Duplicate
				}
			} else {
				entry := w.File{
					FileMeta: m.FileMeta{
						FileId: m.FileId{
							Root: file.Root,
							Path: b.controller.currentPath,
							Name: name,
						},
						Size:    file.Size,
						ModTime: file.ModTime,
					},
					FileKind: w.FileFolder,
				}
				if b.duplicateCounts[file.Hash] > 1 {
					entry.Status = w.Duplicate
				}
				b.controller.entries = append(b.controller.entries, entry)
				log.Printf("### handleOrigin.2: entry %v", entry)
			}
		}
	}
}

func (b screenBuilder) handleCopy(archive *archive) {
	if !b.originHashed {
		return
	}
	log.Printf("### handleCopy ---- ")

	for _, file := range archive.infoByName {
		if file.Hash == "" {
			continue
		}
		if _, ok := b.duplicateCounts[file.Hash]; ok {
			continue
		}
		if _, ok := b.absentHashes[file.Hash]; !ok {
			b.controller.absentFiles++
		}
		if name, ok := b.copyHashName[file.Hash]; ok && file.Name == name {
			continue
		}
		if file.Path == b.controller.currentPath {
			entry := w.File{
				FileMeta: file.FileMeta,
				FileKind: w.FileRegular,
				Hash:     file.Hash,
				Status:   w.Absent,
			}

			log.Printf("### handleCopy.1: entry %v", entry)
			b.controller.entries = append(b.controller.entries, entry)
			b.copyHashName[entry.Hash] = entry.Name
		} else if strings.HasPrefix(file.Path.String(), b.controller.currentPath.String()) {
			relPath := file.Path
			if len(b.controller.currentPath) > 0 {
				relPath = file.Path[len(b.controller.currentPath)+1:]
			}
			name := m.Name(strings.SplitN(relPath.String(), "/", 2)[0])

			_, found := m.Find(b.controller.entries, func(entry w.File) bool { return name == entry.Name })
			if found {
				continue
			}
			entry := w.File{
				FileMeta: m.FileMeta{
					FileId: m.FileId{
						Root: file.Root,
						Path: b.controller.currentPath,
						Name: name,
					},
				},
				FileKind: w.FileFolder,
				Status:   w.Absent,
			}
			b.controller.entries = append(b.controller.entries, entry)
			log.Printf("### handleCopy.2: entry %v", entry)

		}
	}
}

func (c *controller) progress() []w.ProgressInfo {
	// TODO
	return nil
}
