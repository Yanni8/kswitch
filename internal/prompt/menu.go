// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package prompt

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// selectMenu draws opts as a menu driven by the arrow keys and returns
// the chosen Value. Typing narrows the menu down to the options matching
// the typed words; Esc clears the search, or aborts when there is none.
// The cursor starts on opts[initial]. The terminal must be in raw mode.
func selectMenu(in *bufio.Reader, out io.Writer, width int, title string, opts []Option, initial int) (string, error) {
	fmt.Fprint(out, hideCursor)
	defer fmt.Fprint(out, showCursor)

	scr := &screen{out: out, width: width}
	var query []rune
	visible := opts
	cur := initial
	if cur < 0 || cur >= len(visible) {
		cur = 0
	}

	// refilter applies the query, keeping the cursor on the highlighted
	// option when it is still shown.
	refilter := func() {
		selected := ""
		if cur < len(visible) {
			selected = visible[cur].Value
		}
		visible = filterOptions(opts, string(query))
		cur = 0
		for i, o := range visible {
			if o.Value == selected {
				cur = i
				break
			}
		}
	}

	draw := func() {
		lines := []string{title}
		if len(query) > 0 {
			lines = append(lines, dim("search: ")+string(query))
		}
		if len(visible) == 0 {
			lines = append(lines, dim("  (no matches)"))
		}
		for i, o := range visible {
			if i == cur {
				lines = append(lines, bold("> "+o.Label))
			} else {
				lines = append(lines, "  "+o.Label)
			}
		}
		hint := dim("↑/↓ to move, type to search, Enter to confirm")
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
			query = append(query, ev.r)
			refilter()
		case keyBackspace:
			if len(query) > 0 {
				query = query[:len(query)-1]
				refilter()
			}
		case keyClearLine, keyDeleteWord:
			if len(query) > 0 {
				query = query[:0]
				refilter()
			}
		case keyUp:
			if cur > 0 {
				cur--
			}
		case keyDown:
			if cur < len(visible)-1 {
				cur++
			}
		case keyEnter:
			if len(visible) == 0 {
				break
			}
			scr.clear()
			fmt.Fprintf(out, "%s %s\r\n", title, visible[cur].Label)
			return visible[cur].Value, nil
		case keyEscape:
			if len(query) == 0 {
				scr.clear()
				return "", ErrAborted
			}
			query = query[:0]
			refilter()
		case keyAbort:
			scr.clear()
			return "", ErrAborted
		}
		draw()
	}
}

// filterOptions returns the options matching query: every word of it
// has to occur in the option's label (escape sequences aside) or value,
// ignoring case. An empty query matches everything.
func filterOptions(opts []Option, query string) []Option {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return opts
	}
	var matches []Option
	for _, o := range opts {
		text := strings.ToLower(plainText(o.Label) + " " + o.Value)
		ok := true
		for _, w := range words {
			if !strings.Contains(text, w) {
				ok = false
				break
			}
		}
		if ok {
			matches = append(matches, o)
		}
	}
	return matches
}

// plainText drops escape sequences (CSI "ESC [ … final" and SS3
// "ESC O x") from s.
func plainText(s string) string {
	var b strings.Builder
	r := []rune(s)
	for i := 0; i < len(r); i++ {
		if r[i] != 0x1b {
			b.WriteRune(r[i])
			continue
		}
		switch {
		case i+1 < len(r) && r[i+1] == '[':
			i += 2
			for i < len(r) && !(r[i] >= 0x40 && r[i] <= 0x7e) {
				i++
			}
		case i+1 < len(r) && r[i+1] == 'O':
			i += 2
		}
	}
	return b.String()
}
