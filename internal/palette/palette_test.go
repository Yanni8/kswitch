package palette

import (
	"math"
	"testing"
)

// WCAG AA asks for a contrast ratio of at least 4.5 for normal text.
const minContrast = 4.5

func TestTextIsReadableOnEveryColor(t *testing.T) {
	for _, c := range Colors {
		if ratio := Contrast(c.Hex, c.Text()); ratio < minContrast {
			t.Errorf("%s: text %s on %s has contrast %.1f, want at least %.1f", c.Name, c.Text(), c.Hex, ratio, minContrast)
		}
	}
	if c, _ := Find("white"); c.Text() != darkText {
		t.Errorf("white should get dark text, got %s", c.Text())
	}
	if c, _ := Find("red"); c.Text() != lightText {
		t.Errorf("red should get light text, got %s", c.Text())
	}
}

func TestContrast(t *testing.T) {
	if got := Contrast("#000000", "#ffffff"); math.Abs(got-21) > 0.01 {
		t.Errorf("black/white contrast = %.2f, want 21", got)
	}
	if got := Contrast("#5a1515", "#5a1515"); math.Abs(got-1) > 0.0001 {
		t.Errorf("identical colors contrast = %.2f, want 1", got)
	}
	if Contrast("#ffffff", "#000000") != Contrast("#000000", "#ffffff") {
		t.Error("contrast must be symmetric")
	}
}

func TestRGB(t *testing.T) {
	if r, g, b := rgb("#5a1515"); r != 90 || g != 21 || b != 21 {
		t.Errorf("rgb(#5a1515) = %d,%d,%d", r, g, b)
	}
	for _, bad := range []string{"", "5a1515", "#5a15", "#zzzzzz"} {
		if r, g, b := rgb(bad); r != 0 || g != 0 || b != 0 {
			t.Errorf("rgb(%q) = %d,%d,%d, want black", bad, r, g, b)
		}
	}
}

func TestFind(t *testing.T) {
	if c, ok := Find("blue"); !ok || c.Hex != "#1a2f5c" {
		t.Errorf("Find(blue) = %+v, %v", c, ok)
	}
	if _, ok := Find("pink"); ok {
		t.Error("Find(pink) should fail")
	}
}

func TestSwatch(t *testing.T) {
	red, _ := Find("red")
	white, _ := Find("white")

	t.Setenv("COLORTERM", "truecolor")
	if got, want := red.Swatch(), "\x1b[48;2;90;21;21;38;2;240;240;240m Aa \x1b[0m"; got != want {
		t.Errorf("24-bit swatch = %q, want %q", got, want)
	}

	t.Setenv("COLORTERM", "")
	if got, want := red.Swatch(), "\x1b[41;97m Aa \x1b[0m"; got != want {
		t.Errorf("ANSI swatch = %q, want %q", got, want)
	}
	if got, want := white.Swatch(), "\x1b[47;30m Aa \x1b[0m"; got != want {
		t.Errorf("ANSI swatch = %q, want %q", got, want)
	}
}
