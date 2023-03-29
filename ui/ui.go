package ui

type Renderer interface {
	PollEvent() any
	Render(screen Screen)
	Sync()
	Exit()
}

type MouseEvent struct {
	Col, Line int
}

type KeyEvent struct {
	Name string
	Rune rune
}

type ResizeEvent struct {
	Width, Height int
}

type Screen [][]Char

type Char struct {
	Rune  rune
	Style Style
}

type Style int

const (
	StyleDefault Style = iota
	StyleHeader
	StyleAppTitle
	StyleArchiveName
	StyleFile
	StyleFolder
	StyleProgressBar
	StyleArchiveHeader
)
