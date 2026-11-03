// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package session

import (
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/creack/pty"

	"github.com/Yanni8/kubeswitch/internal/terminal"
)

// Run starts cmd inside a pseudo-terminal one row shorter than the
// terminal behind in and out, relays keystrokes and output between the
// two, and keeps banner on the bottom row until cmd exits. If the
// terminal is too small for a banner, cmd runs attached to it directly.
func Run(cmd *exec.Cmd, in, out *os.File, banner Banner) error {
	cols, rows, err := terminal.Size(out)
	if err != nil || rows < minRows {
		return runPlain(cmd, in, out)
	}
	restore, err := terminal.MakeRaw(in)
	if err != nil {
		return runPlain(cmd, in, out)
	}
	defer restore()

	ptmx, err := pty.StartWithSize(cmd, winsize(cols, rows))
	if err != nil {
		return err
	}
	defer ptmx.Close()

	s := newScreen(out, cols, rows, banner)
	s.open(cursorRow(in, out, rows))
	defer s.close()

	if banner.Flash {
		done := make(chan struct{})
		defer close(done)
		go func() {
			ticker := time.NewTicker(flashInterval)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					s.flash()
				}
			}
		}()
	}

	// Keystrokes go to the shell. The copy ends with an error once the
	// pseudo-terminal is closed after the shell has exited.
	go io.Copy(ptmx, in)

	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	defer func() { signal.Stop(winch); close(winch) }()
	go func() {
		for range winch {
			if cols, rows, err := terminal.Size(out); err == nil && rows >= minRows {
				pty.Setsize(ptmx, winsize(cols, rows))
				s.resize(cols, rows)
			}
		}
	}()

	buf := make([]byte, 32*1024)
	for {
		n, err := ptmx.Read(buf)
		if n > 0 {
			s.write(buf[:n])
		}
		if err != nil { // EOF or EIO: the shell has exited
			break
		}
	}
	return cmd.Wait()
}

// winsize is the shell's view of the terminal: one row less than the
// real one.
func winsize(cols, rows int) *pty.Winsize {
	return &pty.Winsize{Rows: uint16(rows - 1), Cols: uint16(cols)}
}

// cursorRow asks the terminal where its cursor is. When it does not
// answer, the bottom row is assumed, which costs at most an extra scroll.
func cursorRow(in, out *os.File, rows int) int {
	if row, _, err := terminal.CursorPosition(in, out); err == nil {
		return row
	}
	return rows
}
