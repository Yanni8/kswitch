package cli

import "fmt"

// backgroundColor is a color every terminal can show as a background:
// the standard ANSI background palette.
type backgroundColor struct {
	Name string
	ansi int // SGR background code
}

var backgroundColors = []backgroundColor{
	{"black", 40},
	{"red", 41},
	{"green", 42},
	{"yellow", 43},
	{"blue", 44},
	{"magenta", 45},
	{"cyan", 46},
	{"white", 47},
}

// swatch renders a block of the color for display in a prompt.
func (c backgroundColor) swatch() string {
	return fmt.Sprintf("\x1b[%dm      \x1b[0m", c.ansi)
}
