package controller

import (
	m "arch/model"
	"log"
)

func (c *controller) handleEvent(event any) {
	if event == nil {
		return
	}

	switch event := event.(type) {
	case m.ArchiveScanned:
		c.archiveScanned(event)

	case m.ArchiveHashed:
		c.archiveHashed(event)

	case m.FileHashed:
		c.fileHashed(event)

	case m.FilesHandled:
		c.filesHandled(event)

	case m.HashingProgress:
		c.handleHashingProgress(event)

	case m.CopyingProgress:
		c.handleCopyingProgress(event)

	case m.ScreenSize:
		c.screenSize = m.ScreenSize{Width: event.Width, Height: event.Height}

	case m.Enter:
		c.enter()

	case m.Esc:
		c.esc()

	case m.RevealInFinder:
		c.revealInFinder()

	case m.MoveSelection:
		c.moveSelection(event.Lines)

	case m.SelectFirst:
		c.selectFirst()

	case m.SelectLast:
		c.selectLast()

	case m.Scroll:
		c.shiftOffset(event.Lines)

	case m.MouseTarget:
		c.mouseTarget(event.Command)

	case m.PgUp:
		c.pgUp()

	case m.PgDn:
		c.pgDn()

	case m.Tab:
		c.tab()

	case m.KeepOne:
		c.keepSelected()

	case m.KeepAll:
		// TODO: Implement, maybe?

	case m.Delete:
		c.deleteSelected()

	case m.Error:
		c.Errors = append(c.Errors, event)

	case m.Quit:
		for hash, presence := range c.state {
			log.Printf("presence: hash: %q, presence: %v", hash, presence)
		}

		c.quit = true

	default:
		log.Panicf("### unhandled event: %#v", event)
	}
}
