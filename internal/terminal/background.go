// Package terminal changes the controlling terminal's background color
// through the OSC 11 escape sequence and restores it afterwards.
package terminal

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// queryTimeout bounds the wait for the terminal to report its current
// background; terminals without OSC 11 support never answer.
const queryTimeout = 300 * time.Millisecond

// Background is a background color change that can be undone.
type Background struct {
	tty      *os.File
	previous string // the color the terminal reported before, "" if unknown
}

// Set changes the terminal's background to color (any OSC 11 color spec,
// e.g. "#cc0000") and remembers the previous one for Restore.
func Set(color string) (*Background, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	previous, _ := query(tty) // best effort; unknown means "reset to default" later
	if _, err := fmt.Fprintf(tty, "\x1b]11;%s\x07", color); err != nil {
		tty.Close()
		return nil, err
	}
	return &Background{tty: tty, previous: previous}, nil
}

// Restore puts the previous background back, or resets it to the
// terminal's default when the previous one is unknown.
func (b *Background) Restore() {
	if b.previous != "" {
		fmt.Fprintf(b.tty, "\x1b]11;%s\x07", b.previous)
	} else {
		fmt.Fprint(b.tty, "\x1b]111\x07")
	}
	b.tty.Close()
}

var errNoReply = errors.New("terminal did not report its background color")

// query asks the terminal for its background color ("OSC 11 ; ?") and
// returns the color spec it answers with.
func query(tty *os.File) (string, error) {
	fd := int(tty.Fd())
	saved, err := term.MakeRaw(fd)
	if err != nil {
		return "", err
	}
	defer term.Restore(fd, saved)

	if _, err := tty.WriteString("\x1b]11;?\x07"); err != nil {
		return "", err
	}

	deadline := time.Now().Add(queryTimeout)
	var reply []byte
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return "", errNoReply
		}
		fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		n, err := unix.Poll(fds, int(remaining.Milliseconds())+1)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return "", err
		}
		if n == 0 {
			return "", errNoReply
		}

		chunk := make([]byte, 128)
		m, err := unix.Read(fd, chunk)
		if err != nil {
			return "", err
		}
		reply = append(reply, chunk[:m]...)
		if spec, ok := parseReply(reply); ok {
			if spec == "" {
				return "", errNoReply
			}
			return spec, nil
		}
	}
}

// parseReply extracts the color spec from a possibly partial OSC 11
// reply such as "ESC ] 11 ; rgb:cccc/0000/0000 BEL". ok is false while
// the reply is still incomplete.
func parseReply(reply []byte) (spec string, ok bool) {
	i := bytes.Index(reply, []byte("]11;"))
	if i < 0 {
		return "", false
	}
	rest := reply[i+4:]
	end := bytes.IndexAny(rest, "\x07\x1b")
	if end < 0 {
		return "", false
	}
	return string(rest[:end]), true
}
