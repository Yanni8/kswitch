// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package prompt

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"unicode"
)

// inputField runs the inline text field: the placeholder is shown dimmed
// until the first character is typed, Enter on an empty field accepts
// it, and Tab or → copies it into the field for editing. Validation
// problems appear under the field. The terminal must be in raw mode.
func inputField(in *bufio.Reader, out io.Writer, width int, title string, o InputOptions) (string, error) {
	scr := &screen{out: out, width: width}
	var text []rune
	problem := ""

	draw := func() {
		var field, hint string
		if len(text) == 0 && o.Placeholder != "" {
			field = "> " + dim(fitPlaceholder(o.Placeholder, width-3))
			hint = dim("Enter to accept the suggestion, Tab to edit it")
		} else {
			field = "> " + string(text)
			hint = dim("Enter to confirm")
		}
		if problem != "" {
			hint = red(problem)
		}
		scr.draw([]string{title, field, hint}, 1, 2+len(text))
	}
	draw()

	for {
		ev, err := readKey(in)
		if err != nil {
			ev.kind = keyAbort
		}
		switch ev.kind {
		case keyRune:
			text = append(text, ev.r)
			problem = ""
		case keyBackspace:
			if len(text) > 0 {
				text = text[:len(text)-1]
				problem = ""
			}
		case keyClearLine:
			text = text[:0]
			problem = ""
		case keyDeleteWord:
			text = deleteWord(text)
			problem = ""
		case keyTab, keyRight:
			if len(text) == 0 {
				text = []rune(o.Placeholder)
				problem = ""
			}
		case keyEnter:
			answer := strings.TrimSpace(string(text))
			if answer == "" {
				answer = o.Placeholder
			}
			p, err := o.validate(answer)
			if err != nil {
				scr.clear()
				return "", err
			}
			if p == "" {
				scr.clear()
				fmt.Fprintf(out, "%s: %s\r\n", title, answer)
				return answer, nil
			}
			problem = p
		case keyAbort:
			scr.clear()
			return "", ErrAborted
		}
		draw()
	}
}

// fitPlaceholder shortens s with an ellipsis so it spans at most max
// columns, keeping the placeholder on a single row.
func fitPlaceholder(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max < 1 {
		return ""
	}
	return string(r[:max-1]) + "…"
}

// deleteWord removes trailing spaces and then the last word.
func deleteWord(text []rune) []rune {
	i := len(text)
	for i > 0 && unicode.IsSpace(text[i-1]) {
		i--
	}
	for i > 0 && !unicode.IsSpace(text[i-1]) {
		i--
	}
	return text[:i]
}
