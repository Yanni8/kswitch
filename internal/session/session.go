// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

// Package session runs a shell inside a pseudo-terminal that is one row
// shorter than the real terminal, keeping the bottom row for a banner.
package session

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Banner is what the session keeps on the bottom row of the terminal.
type Banner struct {
	Text  string // centered on the row
	Short string // used instead when Text does not fit the width
	Style string // SGR sequence the row is drawn in; the alarm style when empty
	Flash bool   // alternate between the style and the second alarm style, once a second
}

// The alarm styles: bold white on red, and bold black on bright yellow
// for the other half of a flash.
const (
	styleAlert = "\x1b[0;1;97;41m"
	styleWarn  = "\x1b[0;1;30;103m"
)

const (
	minRows       = 3                      // the smallest terminal that gets a banner
	flashInterval = 500 * time.Millisecond // half a flash cycle: one flash per second
	flashQuiet    = 150 * time.Millisecond // output pause required before a flash repaints
)

// runPlain runs cmd attached to the terminal, without a banner.
func runPlain(cmd *exec.Cmd, in, out *os.File) error {
	cmd.Stdin, cmd.Stdout, cmd.Stderr = in, out, out
	return cmd.Run()
}

// screen drives the real terminal: it keeps the scroll region above the
// banner row and paints the banner.
type screen struct {
	mu         sync.Mutex
	out        io.Writer
	cols, rows int
	banner     Banner
	phase      int       // which alarm style is showing
	lastOutput time.Time // when the shell last wrote something
	closed     bool
	filter     filter
}

func newScreen(out io.Writer, cols, rows int, banner Banner) *screen {
	s := &screen{out: out, cols: cols, rows: rows, banner: banner}
	s.filter = filter{rows: rows, repaint: s.paint()}
	return s
}

// paint is the sequence that re-asserts the scroll region and draws the
// banner, leaving the cursor where it was.
func (s *screen) paint() string {
	return fmt.Sprintf("\x1b7\x1b[1;%dr\x1b[%d;1H%s\x1b8", s.rows-1, s.rows, s.line())
}

// line renders the banner centered across the whole width in the
// current alarm style.
func (s *screen) line() string {
	style := s.banner.Style
	if style == "" {
		style = styleAlert
	}
	if s.banner.Flash && s.phase%2 == 1 {
		style = styleWarn
	}
	text := []rune(s.banner.Text)
	if len(text) > s.cols && s.banner.Short != "" {
		text = []rune(s.banner.Short)
	}
	if len(text) > s.cols {
		text = text[:s.cols]
	}
	left := (s.cols - len(text)) / 2
	right := s.cols - len(text) - left
	return style + strings.Repeat(" ", left) + string(text) + strings.Repeat(" ", right) + "\x1b[0m"
}

// open takes the bottom row and puts the cursor on row, inside the
// region, for the shell's first prompt. If that is the banner row, the
// screen scrolls up a line first.
func (s *screen) open(row int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if row >= s.rows {
		fmt.Fprint(s.out, "\n")
		row = s.rows - 1
	}
	if row < 1 {
		row = 1
	}
	fmt.Fprintf(s.out, "%s\x1b[%d;1H", s.paint(), row)
}

// write passes shell output through the filter to the terminal.
func (s *screen) write(chunk []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastOutput = time.Now()
	s.out.Write(s.filter.process(chunk))
}

// flash switches to the other alarm style and repaints. It skips a beat
// while the shell is producing output, so the repaint cannot land
// between a program's own cursor save and restore.
func (s *screen) flash() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || time.Since(s.lastOutput) < flashQuiet {
		return
	}
	s.phase++
	s.filter.repaint = s.paint()
	io.WriteString(s.out, s.filter.repaint)
}

// resize follows a new terminal size.
func (s *screen) resize(cols, rows int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cols, s.rows = cols, rows
	s.filter.rows, s.filter.repaint = rows, s.paint()
	io.WriteString(s.out, s.filter.repaint)
}

// close gives the whole screen back, clears the banner row, and leaves
// the cursor on it so whatever follows appears below the shell's output.
func (s *screen) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.out.Write(s.filter.flush())
	fmt.Fprintf(s.out, "\x1b[r\x1b[%d;1H\x1b[2K\x1b[0m", s.rows)
}
