package cli

import "fmt"

// backgroundColor is a color every terminal can show as a background:
// the standard ANSI background palette.
type backgroundColor struct {
	Name string
	ansi int    // SGR background code, used for swatches
	hex  string // what the terminal background is set to (OSC 11)
}

// The shades follow the classic Tango palette used by many terminals:
// recognizably the named colors, but easier on the eyes than pure RGB.
var backgroundColors = []backgroundColor{
	{"black", 40, "#000000"},
	{"red", 41, "#cc0000"},
	{"green", 42, "#4e9a06"},
	{"yellow", 43, "#c4a000"},
	{"blue", 44, "#3465a4"},
	{"magenta", 45, "#75507b"},
	{"cyan", 46, "#06989a"},
	{"white", 47, "#d3d7cf"},
}

// swatch renders a block of the color for display in a prompt.
func (c backgroundColor) swatch() string {
	return fmt.Sprintf("\x1b[%dm      \x1b[0m", c.ansi)
}

// findColor looks a palette color up by name.
func findColor(name string) (backgroundColor, bool) {
	for _, c := range backgroundColors {
		if c.Name == name {
			return c, true
		}
	}
	return backgroundColor{}, false
}

// colorSwatch renders a block in the named color, or "" if the name is
// not in the palette.
func colorSwatch(name string) string {
	c, ok := findColor(name)
	if !ok {
		return ""
	}
	return c.swatch()
}
