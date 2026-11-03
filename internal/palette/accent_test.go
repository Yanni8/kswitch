// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package palette

import (
	"strings"
	"testing"
)

func TestAccentsAreReadableAndDistinct(t *testing.T) {
	for _, c := range Colors {
		accent := c.Accent()
		if accent == c.Hex {
			t.Errorf("%s: the accent must differ from the background", c.Name)
		}
		if ratio := Contrast(accent, TextOn(accent)); ratio < minContrast {
			t.Errorf("%s: text on the accent %s has contrast %.1f, want at least %.1f", c.Name, accent, ratio, minContrast)
		}
		lighter := luminance(accent) > luminance(c.Hex)
		if light := luminance(c.Hex) > 0.5; light == lighter {
			t.Errorf("%s: a %s shade should get a %s accent, got %s", c.Name,
				map[bool]string{true: "light", false: "dark"}[light],
				map[bool]string{true: "darker", false: "lighter"}[light], accent)
		}
	}
	if ratio := Contrast(NeutralAccent, TextOn(NeutralAccent)); ratio < minContrast {
		t.Errorf("the neutral accent has contrast %.1f, want at least %.1f", ratio, minContrast)
	}
	if green, _ := Find("green"); green.Accent() != "#98af98" {
		t.Errorf("green accent = %s, want the sage #98af98", green.Accent())
	}
}

func TestStyle(t *testing.T) {
	t.Setenv("COLORTERM", "truecolor")
	if got, want := Style("#98af98", "7"), "\x1b[0;48;2;152;175;152;38;2;26;26;26m"; got != want {
		t.Errorf("24-bit style = %q, want %q", got, want)
	}

	t.Setenv("COLORTERM", "")
	if got := Style("#98af98", "7"); got != "\x1b[0;7m" {
		t.Errorf("fallback style = %q, want reverse video", got)
	}
	if !strings.HasPrefix(Style(NeutralAccent, "7"), "\x1b[0;") {
		t.Error("styles always start by resetting attributes")
	}
}
