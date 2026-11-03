// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"bytes"
	"strings"
	"testing"
)

func TestBannerStyle(t *testing.T) {
	const custom = "\x1b[0;48;2;152;175;152;38;2;26;26;26m"
	quiet := newScreen(&bytes.Buffer{}, 12, 5, Banner{Text: "Local", Style: custom})
	if got := quiet.line(); !strings.HasPrefix(got, custom) || !strings.Contains(got, "Local") {
		t.Errorf("a banner should be drawn in its own style: %q", got)
	}
	quiet.phase = 1 // without Flash the phase changes nothing
	if got := quiet.line(); !strings.HasPrefix(got, custom) {
		t.Errorf("a banner that does not flash keeps its style: %q", got)
	}

	alarm := newScreen(&bytes.Buffer{}, 12, 5, Banner{Text: "PROD", Flash: true})
	if got := alarm.line(); !strings.HasPrefix(got, styleAlert) {
		t.Errorf("without a style the alarm style is used: %q", got)
	}
	alarm.phase = 1
	if got := alarm.line(); !strings.HasPrefix(got, styleWarn) {
		t.Errorf("a flashing banner alternates with the second alarm style: %q", got)
	}
}
