// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package prompt

import (
	"bufio"
	"fmt"
	"io"
)

// selectMenu draws opts as a menu driven by the arrow keys (or j/k) with
// the cursor starting on opts[initial], and returns the chosen Value.
// The terminal must be in raw mode.
func selectMenu(in *bufio.Reader, out io.Writer, width int, title string, opts []Option, initial int) (string, error) {
	fmt.Fprint(out, hideCursor)
	defer fmt.Fprint(out, showCursor)

	scr := &screen{out: out, width: width}
	cur := initial
	draw := func() {
		lines := make([]string, 0, len(opts)+2)
		lines = append(lines, title)
		for i, o := range opts {
			if i == cur {
				lines = append(lines, bold("> "+o.Label))
			} else {
				lines = append(lines, "  "+o.Label)
			}
		}
		hint := dim("↑/↓ to move, Enter to confirm")
		lines = append(lines, hint)
		scr.draw(lines, len(lines)-1, visibleWidth(hint))
	}
	draw()

	for {
		ev, err := readKey(in)
		if err != nil {
			ev.kind = keyAbort
		}
		switch ev.kind {
		case keyRune:
			switch ev.r {
			case 'k':
				ev.kind = keyUp
			case 'j':
				ev.kind = keyDown
			case 'q':
				ev.kind = keyAbort
			}
		}
		switch ev.kind {
		case keyUp:
			if cur > 0 {
				cur--
			}
		case keyDown:
			if cur < len(opts)-1 {
				cur++
			}
		case keyEnter:
			scr.clear()
			fmt.Fprintf(out, "%s %s\r\n", title, opts[cur].Label)
			return opts[cur].Value, nil
		case keyAbort:
			scr.clear()
			return "", ErrAborted
		}
		draw()
	}
}
