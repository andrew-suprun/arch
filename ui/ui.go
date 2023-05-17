package ui

import "time"

type Position struct {
	X int
	Y int
}

type Size struct {
	Width  int
	Height int
}

type Flex struct {
	X int
	Y int
}

type Constraint struct {
	Size
	Flex
}

type MouseEvent struct {
	Position
	Button
	ButtonModifier
	time.Time
}

type Button int

const (
	LeftButton  Button = 1
	RightButton Button = 2
)

type ButtonModifier int

const (
	Shift   ButtonModifier = 1
	Control ButtonModifier = 2
	Option  ButtonModifier = 4
)

type ScrollEvent struct {
	Direction
}

type Direction int

const (
	ScrollUp   Direction = 1
	ScrollDown Direction = 2
)

type KeyEvent struct {
	Name string
	Rune rune
}

type ResizeEvent struct {
	Width, Height int
}
