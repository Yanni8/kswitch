// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package terminal

import (
	"errors"
	"os"
	"regexp"
	"strconv"
	"time"
)

var (
	cursorReply      = regexp.MustCompile(`\x1b\[(\d+);(\d+)R`)
	errNoCursorReply = errors.New("terminal did not report its cursor position")
)

// CursorPosition asks the terminal behind in and out where its cursor is
// (1-based row and column) and waits briefly for the answer.
func CursorPosition(in, out *os.File) (row, col int, err error) {
	restore, err := MakeRaw(in)
	if err != nil {
		return 0, 0, err
	}
	defer restore()

	if _, err := out.WriteString("\x1b[6n"); err != nil {
		return 0, 0, err
	}

	deadline := time.Now().Add(queryTimeout)
	var reply []byte
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return 0, 0, errNoCursorReply
		}
		var ready bool
		err := withFD(in, func(fd int) error {
			var err error
			ready, err = waitReadable(fd, remaining)
			return err
		})
		if err != nil {
			return 0, 0, err
		}
		if !ready {
			return 0, 0, errNoCursorReply
		}

		chunk := make([]byte, 64)
		n, err := in.Read(chunk)
		if err != nil {
			return 0, 0, err
		}
		reply = append(reply, chunk[:n]...)
		if m := cursorReply.FindSubmatch(reply); m != nil {
			row, _ = strconv.Atoi(string(m[1]))
			col, _ = strconv.Atoi(string(m[2]))
			return row, col, nil
		}
	}
}
