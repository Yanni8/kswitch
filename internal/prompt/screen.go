package prompt

import (
	"fmt"
	"io"
)

const (
	hideCursor = "\x1b[?25l"
	showCursor = "\x1b[?25h"
)

func bold(s string) string { return "\x1b[1m" + s + "\x1b[0m" }
func dim(s string) string  { return "\x1b[2m" + s + "\x1b[0m" }
func red(s string) string  { return "\x1b[31m" + s + "\x1b[0m" }

// screen redraws a block of lines in place on a raw-mode terminal. It
// remembers where the cursor was parked so the block can be found again
// with relative cursor movement, which keeps working when lines wrap or
// the terminal scrolls.
type screen struct {
	out    io.Writer
	width  int
	parked int // rows from the block's first row down to the cursor
}

// draw replaces the block with lines and parks the cursor on lines[line]
// at logical column col (columns from that line's start, ignoring wraps).
func (s *screen) draw(lines []string, line, col int) {
	s.clear()
	total := 0
	starts := make([]int, len(lines))
	for i, l := range lines {
		starts[i] = total
		if i > 0 {
			fmt.Fprint(s.out, "\r\n")
		}
		fmt.Fprint(s.out, l)
		total += s.rows(visibleWidth(l))
	}

	// The cursor now sits on the block's last row. A column that is an
	// exact multiple of the width stays on its row, at the last column.
	row, x := starts[line], col
	if col > 0 {
		wraps := (col - 1) / s.width
		row += wraps
		x = col - wraps*s.width
	}
	if up := total - 1 - row; up > 0 {
		fmt.Fprintf(s.out, "\x1b[%dA", up)
	}
	fmt.Fprint(s.out, "\r")
	if x > 0 {
		fmt.Fprintf(s.out, "\x1b[%dC", x)
	}
	s.parked = row
}

// clear wipes the block and leaves the cursor at its first row.
func (s *screen) clear() {
	if s.parked > 0 {
		fmt.Fprintf(s.out, "\x1b[%dA", s.parked)
	}
	fmt.Fprint(s.out, "\r\x1b[J")
	s.parked = 0
}

// rows is how many terminal rows a line of n visible columns occupies.
func (s *screen) rows(n int) int {
	if n <= 0 {
		return 1
	}
	return (n-1)/s.width + 1
}

// visibleWidth counts the columns s occupies, skipping escape sequences
// (CSI "ESC [ … final" and SS3 "ESC O x"). Every printable rune is
// assumed to be one column wide.
func visibleWidth(s string) int {
	n := 0
	r := []rune(s)
	for i := 0; i < len(r); i++ {
		if r[i] != 0x1b {
			n++
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
	return n
}
