// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

// Package palette defines the background colors a kubeconfig can be
// given and the text color that reads well on each of them.
package palette

import (
	"fmt"
	"math"
	"os"
	"strconv"
)

// Color is a background color from the palette.
type Color struct {
	Name string
	Hex  string // the terminal background, "#rrggbb"
	ansi int    // SGR background code: the swatch fallback without 24-bit color
}

// Colors are the choices offered when adding a kubeconfig. The shades
// are deep enough that light text and the usual ANSI-colored output stay
// readable on a whole terminal in them, yet unmistakable; white is the
// one light option and gets dark text.
var Colors = []Color{
	{"black", "#000000", 40},
	{"red", "#5a1515", 41},
	{"green", "#1b4d1b", 42},
	{"yellow", "#6b5e00", 43},
	{"blue", "#1a2f5c", 44},
	{"magenta", "#4d1a4d", 45},
	{"cyan", "#0f4a4d", 46},
	{"white", "#e6e6e6", 47},
}

const (
	lightText = "#f0f0f0"
	darkText  = "#1a1a1a"
)

// Find looks a color up by name.
func Find(name string) (Color, bool) {
	for _, c := range Colors {
		if c.Name == name {
			return c, true
		}
	}
	return Color{}, false
}

// Text returns the text color that reads best on the background: light
// on dark shades, dark on light ones.
func (c Color) Text() string {
	if Contrast(c.Hex, lightText) >= Contrast(c.Hex, darkText) {
		return lightText
	}
	return darkText
}

// Swatch previews text on the background, for menus and lists. Without
// known 24-bit color support it falls back to the nearest ANSI colors.
func (c Color) Swatch() string {
	if trueColor() {
		return fmt.Sprintf("\x1b[%s;%sm Aa \x1b[0m", sgr(48, c.Hex), sgr(38, c.Text()))
	}
	text := 97 // bright white
	if c.Text() == darkText {
		text = 30 // black
	}
	return fmt.Sprintf("\x1b[%d;%dm Aa \x1b[0m", c.ansi, text)
}

func trueColor() bool {
	switch os.Getenv("COLORTERM") {
	case "truecolor", "24bit":
		return true
	}
	return false
}

// sgr builds the 24-bit color part of an SGR sequence; 38 selects the
// text color and 48 the background.
func sgr(code int, hex string) string {
	r, g, b := rgb(hex)
	return fmt.Sprintf("%d;2;%d;%d;%d", code, r, g, b)
}

// Contrast returns the WCAG contrast ratio between two "#rrggbb" colors:
// 1 for identical colors, 21 for black and white.
func Contrast(a, b string) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// luminance is the relative luminance of a "#rrggbb" color as defined
// by WCAG.
func luminance(hex string) float64 {
	r, g, b := rgb(hex)
	return 0.2126*linear(r) + 0.7152*linear(g) + 0.0722*linear(b)
}

func linear(c uint8) float64 {
	v := float64(c) / 255
	if v <= 0.03928 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

// rgb parses "#rrggbb"; malformed input yields black.
func rgb(hex string) (r, g, b uint8) {
	if len(hex) != 7 || hex[0] != '#' {
		return 0, 0, 0
	}
	v, err := strconv.ParseUint(hex[1:], 16, 32)
	if err != nil {
		return 0, 0, 0
	}
	return uint8(v >> 16), uint8(v >> 8), uint8(v)
}
