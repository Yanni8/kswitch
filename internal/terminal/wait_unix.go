// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package terminal

import (
	"errors"
	"time"

	"golang.org/x/sys/unix"
)

// waitReadable blocks until fd has input to read or timeout passes, and
// reports which of the two happened.
func waitReadable(fd int, timeout time.Duration) (bool, error) {
	for {
		fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		n, err := unix.Poll(fds, int(timeout.Milliseconds())+1)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return false, err
		}
		return n > 0, nil
	}
}
