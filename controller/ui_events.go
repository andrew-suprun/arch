package controller

import (
	m "arch/model"
	w "arch/widgets"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (c *controller) mouseTarget(cmd any) {
	folder := c.currentFolder()
	switch cmd := cmd.(type) {
	case m.SelectFile:
		if c.getSelectedId() == m.Id(cmd) && time.Since(c.lastMouseEventTime).Seconds() < 0.5 {
			c.open()
		} else {
			c.setSelectedId(m.Id(cmd))
		}
		c.lastMouseEventTime = time.Now()

	case m.SelectFolder:
		c.currentPath = m.Path(cmd)

	case w.SortColumn:
		if cmd == folder.sortColumn {
			folder.sortAscending[folder.sortColumn] = !folder.sortAscending[folder.sortColumn]
		} else {
			folder.sortColumn = cmd
		}
	}
}

func (c *controller) selectFirst() {
	if len(c.screen.Entries) > 0 {
		c.setSelectedIdx(0)
		c.currentFolder().offsetIdx = 0
	}
}

func (c *controller) selectLast() {
	if len(c.screen.Entries) > 0 {
		c.setSelectedIdx(len(c.screen.Entries) - 1)
		c.makeSelectedVisible()
	}
}

func (c *controller) open() {
	exec.Command("open", c.getSelectedId().String()).Start()
}

func (c *controller) enter() {
	selectedId := c.getSelectedId()
	var file *w.File
	for i := range c.screen.Entries {
		if c.screen.Entries[i].Id == selectedId {
			file = c.screen.Entries[i]
			break
		}
	}

	if file != nil && file.Kind == w.FileFolder {
		c.currentPath = m.Path(file.Name.String())
	}
}

func (c *controller) pgUp() {
	c.shiftOffset(-c.screen.FileTreeLines)
	c.moveSelection(-c.screen.FileTreeLines)
}

func (c *controller) pgDn() {
	c.shiftOffset(c.screen.FileTreeLines)
	c.moveSelection(c.screen.FileTreeLines)
}

func (c *controller) exit() {
	if c.currentPath == "" {
		return
	}
	parts := strings.Split(c.currentPath.String(), "/")
	if len(parts) == 1 {
		c.currentPath = ""
	}
	c.currentPath = m.Path(filepath.Join(parts[:len(parts)-1]...))
}

func (c *controller) revealInFinder() {
	exec.Command("open", "-R", c.getSelectedId().String()).Start()
}

func (c *controller) moveSelection(lines int) {
	c.setSelectedIdx(c.getSelectedIdx() + lines)
	c.makeSelectedVisible()
}

func (c *controller) shiftOffset(lines int) {
	folder := c.currentFolder()
	folder.offsetIdx += lines
	if folder.offsetIdx < 0 {
		folder.offsetIdx = 0
	} else if folder.offsetIdx >= len(c.screen.Entries) {
		folder.offsetIdx = len(c.screen.Entries) - 1
	}
}

func (c *controller) tab() {
	selected := c.getSelectedFile()

	if selected == nil || selected.Kind != w.FileRegular || c.state[selected.Hash] != w.Duplicate {
		return
	}
	sameHash := []m.Id{}
	c.do(func(file *m.File) bool {
		if file.Hash == selected.Hash && file.Root == c.origin {
			sameHash = append(sameHash, file.Id)
		}
		return true
	})
	sort.Slice(sameHash, func(i, j int) bool {
		return strings.ToLower(sameHash[i].Name.String()) < strings.ToLower(sameHash[j].Name.String())
	})

	idx, _ := m.Find(sameHash, func(id m.Id) bool { return id == selected.Id })
	idx++
	if idx == len(sameHash) {
		idx = 0
	}
	id := sameHash[idx]
	c.currentPath = id.Path
	c.setSelectedId(id)

	c.makeSelectedVisible()
}

func (c *controller) makeSelectedVisible() {
	selectedIdx := c.getSelectedIdx()
	offsetIdx := c.currentFolder().offsetIdx

	if offsetIdx > selectedIdx {
		offsetIdx = selectedIdx
	}
	if offsetIdx < selectedIdx+1-c.screen.FileTreeLines {
		offsetIdx = selectedIdx + 1 - c.screen.FileTreeLines
	}

	c.currentFolder().offsetIdx = offsetIdx
}
