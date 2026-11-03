// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package prompt

import (
	"bufio"
	"unicode"
)

type key int

const (
	keyNone key = iota
	keyRune     // a printable character, in keyEvent.r
	keyEnter
	keyTab
	keyBackspace
	keyClearLine  // Ctrl-U
	keyDeleteWord // Ctrl-W
	keyUp
	keyDown
	keyLeft
	keyRight
	keyEscape // a lone Esc
	keyAbort  // Ctrl-C, Ctrl-D
)

type keyEvent struct {
	kind key
	r    rune
}

// readKey reads one keypress from a raw-mode terminal.
func readKey(in *bufio.Reader) (keyEvent, error) {
	r, _, err := in.ReadRune()
	if err != nil {
		return keyEvent{kind: keyAbort}, err
	}
	switch r {
	case '\r', '\n':
		return keyEvent{kind: keyEnter}, nil
	case '\t':
		return keyEvent{kind: keyTab}, nil
	case 0x7f, 0x08:
		return keyEvent{kind: keyBackspace}, nil
	case 0x03, 0x04:
		return keyEvent{kind: keyAbort}, nil
	case 0x15:
		return keyEvent{kind: keyClearLine}, nil
	case 0x17:
		return keyEvent{kind: keyDeleteWord}, nil
	case 0x1b:
		return readEscape(in)
	}
	if unicode.IsPrint(r) {
		return keyEvent{kind: keyRune, r: r}, nil
	}
	return keyEvent{kind: keyNone}, nil
}

// readEscape parses what follows an ESC byte. Arrow keys arrive as
// CSI sequences ("ESC [ A") or, in application mode, SS3 ("ESC O A").
// A sequence is delivered in one read, so a lone Esc keypress has
// nothing buffered behind it.
func readEscape(in *bufio.Reader) (keyEvent, error) {
	if in.Buffered() == 0 {
		return keyEvent{kind: keyEscape}, nil
	}
	lead, err := in.ReadByte()
	if err != nil {
		return keyEvent{kind: keyAbort}, err
	}

	var final byte
	switch lead {
	case '[': // parameter (0x30-0x3F) and intermediate (0x20-0x2F) bytes, then a final byte
		for {
			b, err := in.ReadByte()
			if err != nil {
				return keyEvent{kind: keyAbort}, err
			}
			if b >= 0x40 && b <= 0x7e {
				final = b
				break
			}
			if b < 0x20 || b > 0x3f {
				return keyEvent{kind: keyNone}, nil
			}
		}
	case 'O':
		if final, err = in.ReadByte(); err != nil {
			return keyEvent{kind: keyAbort}, err
		}
	default: // Alt+key and the like
		return keyEvent{kind: keyNone}, nil
	}

	switch final {
	case 'A':
		return keyEvent{kind: keyUp}, nil
	case 'B':
		return keyEvent{kind: keyDown}, nil
	case 'C':
		return keyEvent{kind: keyRight}, nil
	case 'D':
		return keyEvent{kind: keyLeft}, nil
	}
	return keyEvent{kind: keyNone}, nil
}
