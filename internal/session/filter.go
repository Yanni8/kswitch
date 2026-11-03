// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"fmt"
	"strconv"
	"strings"
)

// maxSequence bounds how many bytes of an unfinished escape sequence are
// held back while waiting for the rest; anything longer is passed on.
const maxSequence = 4096

// filter watches the shell's output for sequences that would disturb the
// banner. Scroll-region settings are confined to the rows above it, and
// the banner is repainted right after sequences that erase the screen,
// switch the alternate screen, or reset the terminal. A sequence split
// across two reads is held back until it is complete.
type filter struct {
	rows    int    // rows of the real terminal; the banner is on the last one
	repaint string // sequence that repaints the banner in place
	pending []byte // start of an escape sequence waiting for its end
}

// process returns chunk ready for the terminal.
func (f *filter) process(chunk []byte) []byte {
	data := append(f.pending, chunk...)
	f.pending = nil
	out := make([]byte, 0, len(data)+len(f.repaint))
	for i := 0; i < len(data); {
		if data[i] != 0x1b {
			out = append(out, data[i])
			i++
			continue
		}
		seq, complete := escapeSequence(data[i:])
		if !complete {
			if len(data)-i < maxSequence {
				f.pending = append([]byte(nil), data[i:]...)
				return out
			}
			return append(out, data[i:]...)
		}
		out = append(out, f.rewrite(seq)...)
		i += len(seq)
	}
	return out
}

// flush returns whatever is held back, for when no more output follows.
func (f *filter) flush() []byte {
	p := f.pending
	f.pending = nil
	return p
}

// escapeSequence returns the escape sequence at the start of b, which
// begins with ESC, and whether it is complete.
func escapeSequence(b []byte) ([]byte, bool) {
	if len(b) < 2 {
		return b, false
	}
	switch b[1] {
	case '[': // CSI: parameter and intermediate bytes, then a final byte
		for j := 2; j < len(b); j++ {
			switch {
			case b[j] >= 0x40 && b[j] <= 0x7e:
				return b[:j+1], true
			case b[j] < 0x20 || b[j] > 0x3f: // not a sequence after all
				return b[:j], true
			}
		}
		return b, false
	case ']', 'P', '^', '_': // OSC, DCS, PM, APC run until BEL or ST
		for j := 2; j < len(b); j++ {
			if b[j] == 0x07 {
				return b[:j+1], true
			}
			if b[j] == 0x1b && j+1 < len(b) && b[j+1] == '\\' {
				return b[:j+2], true
			}
		}
		return b, false
	}
	if b[1] >= 0x20 && b[1] <= 0x2f { // intermediate byte, e.g. "ESC ( B"
		for j := 2; j < len(b); j++ {
			if b[j] >= 0x30 && b[j] <= 0x7e {
				return b[:j+1], true
			}
			if b[j] < 0x20 || b[j] > 0x2f {
				return b[:j], true
			}
		}
		return b, false
	}
	return b[:2], true
}

// rewrite adjusts one complete escape sequence.
func (f *filter) rewrite(seq []byte) []byte {
	if len(seq) == 2 && seq[1] == 'c' { // RIS: the terminal forgets the region
		return f.withRepaint(seq)
	}
	if len(seq) < 3 || seq[1] != '[' {
		return seq
	}
	final := seq[len(seq)-1]
	params := string(seq[2 : len(seq)-1])
	switch final {
	case 'r': // DECSTBM: keep the scroll region above the banner
		top, bottom := 1, f.rows-1
		t, b, _ := strings.Cut(params, ";")
		if n, err := strconv.Atoi(t); err == nil && n > 0 {
			top = n
		}
		if n, err := strconv.Atoi(b); err == nil && n > 0 && n < f.rows {
			bottom = n
		}
		if top >= bottom {
			top, bottom = 1, f.rows-1
		}
		return []byte(fmt.Sprintf("\x1b[%d;%dr", top, bottom))
	case 'J': // erase in display: all but "above the cursor" reach the banner
		if params == "" || params == "0" || params == "2" || params == "3" {
			return f.withRepaint(seq)
		}
	case 'h', 'l': // alternate screen in or out
		if strings.HasPrefix(params, "?") && (strings.Contains(params, "1049") || strings.Contains(params, "1047") || params == "?47") {
			return f.withRepaint(seq)
		}
	}
	return seq
}

// withRepaint returns seq followed by the repaint sequence, in a fresh
// slice so the caller's buffer is left alone.
func (f *filter) withRepaint(seq []byte) []byte {
	out := make([]byte, 0, len(seq)+len(f.repaint))
	out = append(out, seq...)
	return append(out, f.repaint...)
}
