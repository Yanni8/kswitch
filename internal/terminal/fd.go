// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package terminal

import (
	"os"

	"golang.org/x/term"
)

// withFD runs fn with f's file descriptor. Unlike f.Fd(), it leaves f in
// non-blocking mode, so closing f still wakes up a pending Read.
func withFD(f *os.File, fn func(fd int) error) error {
	rc, err := f.SyscallConn()
	if err != nil {
		return err
	}
	var inner error
	if err := rc.Control(func(fd uintptr) { inner = fn(int(fd)) }); err != nil {
		return err
	}
	return inner
}

// Size returns the size of the terminal behind f in columns and rows.
func Size(f *os.File) (cols, rows int, err error) {
	err = withFD(f, func(fd int) error {
		var err error
		cols, rows, err = term.GetSize(fd)
		return err
	})
	return cols, rows, err
}

// MakeRaw puts the terminal behind f into raw mode and returns the
// function that restores its previous state.
func MakeRaw(f *os.File) (restore func() error, err error) {
	var saved *term.State
	err = withFD(f, func(fd int) error {
		var err error
		saved, err = term.MakeRaw(fd)
		return err
	})
	if err != nil {
		return nil, err
	}
	return func() error {
		return withFD(f, func(fd int) error { return term.Restore(fd, saved) })
	}, nil
}
