// Package terminal changes the controlling terminal's text and
// background colors through the OSC 10 and OSC 11 escape sequences and
// restores them afterwards.
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

const (
	oscText       = 10
	oscBackground = 11
)

// queryTimeout bounds the wait for the terminal to report a color;
// terminals without OSC 10/11 support never answer.
const queryTimeout = 300 * time.Millisecond

// Colors is a change of the terminal's text and background colors that
// can be undone with Restore.
type Colors struct {
	tty            *os.File
	prevText       string // as reported by the terminal, "" if unknown
	prevBackground string
}

// SetColors changes the terminal's text and background colors (any OSC
// color spec, e.g. "#f0f0f0") and remembers the previous ones.
func SetColors(text, background string) (*Colors, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	c := &Colors{tty: tty}
	// Best effort: colors the terminal does not report are reset to its
	// defaults by Restore instead. The second query is skipped when the
	// first one times out, so the wait is paid only once.
	if c.prevBackground, err = query(tty, oscBackground); err == nil {
		c.prevText, _ = query(tty, oscText)
	}
	if _, err := fmt.Fprint(tty, set(oscText, text)+set(oscBackground, background)); err != nil {
		tty.Close()
		return nil, err
	}
	return c, nil
}

// Restore puts the previous colors back, or resets them to the
// terminal's defaults when they are unknown.
func (c *Colors) Restore() {
	fmt.Fprint(c.tty, restore(oscText, c.prevText)+restore(oscBackground, c.prevBackground))
	c.tty.Close()
}

func set(code int, color string) string {
	return fmt.Sprintf("\x1b]%d;%s\x07", code, color)
}

func restore(code int, previous string) string {
	if previous != "" {
		return set(code, previous)
	}
	return fmt.Sprintf("\x1b]%d\x07", code+100) // OSC 110 / 111 reset to the default
}

var errNoReply = errors.New("terminal did not report the color")

// query asks the terminal for the color behind an OSC code ("OSC n ; ?")
// and returns the color spec it answers with.
func query(tty *os.File, code int) (string, error) {
	fd := int(tty.Fd())
	saved, err := term.MakeRaw(fd)
	if err != nil {
		return "", err
	}
	defer term.Restore(fd, saved)

	if _, err := tty.WriteString(set(code, "?")); err != nil {
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
		if spec, ok := parseReply(reply, code); ok {
			if spec == "" {
				return "", errNoReply
			}
			return spec, nil
		}
	}
}

// parseReply extracts the color spec from a possibly partial reply such
// as "ESC ] 11 ; rgb:cccc/0000/0000 BEL". ok is false while the reply
// is still incomplete.
func parseReply(reply []byte, code int) (spec string, ok bool) {
	i := bytes.Index(reply, []byte(fmt.Sprintf("]%d;", code)))
	if i < 0 {
		return "", false
	}
	rest := reply[i+len(fmt.Sprintf("]%d;", code)):]
	end := bytes.IndexAny(rest, "\x07\x1b")
	if end < 0 {
		return "", false
	}
	return string(rest[:end]), true
}
