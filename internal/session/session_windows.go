// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package session

import (
	"os"
	"os/exec"
)

// Run runs cmd attached to the terminal; keeping a banner row is not
// supported on Windows.
func Run(cmd *exec.Cmd, in, out *os.File, _ Banner) error {
	return runPlain(cmd, in, out)
}
