// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package session

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
)

// fakeTerminal is a pseudo-terminal standing in for the user's terminal:
// the session drives tty, and everything it writes shows up on master.
type fakeTerminal struct {
	master, tty *os.File
	mu          sync.Mutex
	out         bytes.Buffer
	done        chan struct{}
}

func newFakeTerminal(t *testing.T, cols, rows int) *fakeTerminal {
	t.Helper()
	master, tty, err := pty.Open()
	if err != nil {
		t.Skipf("no pseudo-terminal available: %v", err)
	}
	// Sized through the master: touching the tty's descriptor directly
	// would switch it to blocking mode, and closing it would then no
	// longer wake up the session's pending read of it.
	if err := pty.Setsize(master, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}); err != nil {
		t.Fatal(err)
	}
	ft := &fakeTerminal{master: master, tty: tty, done: make(chan struct{})}
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := master.Read(buf)
			ft.mu.Lock()
			ft.out.Write(buf[:n])
			ft.mu.Unlock()
			if err != nil {
				break
			}
		}
		close(ft.done)
	}()
	t.Cleanup(func() { tty.Close(); master.Close() })
	return ft
}

func (ft *fakeTerminal) output() string {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	return ft.out.String()
}

// finish closes the terminal and returns everything written to it.
func (ft *fakeTerminal) finish() string {
	ft.tty.Close()
	<-ft.done
	return ft.output()
}

// waitFor polls the terminal output until it contains want or the time
// runs out.
func (ft *fakeTerminal) waitFor(t *testing.T, want string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(ft.output(), want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("output never contained %q:\n%q", want, ft.output())
}

func TestRunPinsBannerAndGivesScreenBack(t *testing.T) {
	ft := newFakeTerminal(t, 80, 24)
	cmd := exec.Command("sh", "-c", "echo hello; printf '\\033[2J'; echo done")
	if err := Run(cmd, ft.tty, ft.tty, Banner{Text: "PROD demo"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := ft.finish()

	region := "\x1b[1;23r"
	for _, want := range []string{
		region,
		"\x1b[24;1H" + styleAlert, // the banner on the bottom row
		"PROD demo",
		"hello",
		"done",
		"\x1b[2J\x1b7" + region,   // repainted right after the screen was erased
		"\x1b[r\x1b[24;1H\x1b[2K", // the whole screen given back at the end
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%q", want, out)
		}
	}
	if strings.Contains(out, styleWarn) {
		t.Errorf("a banner that does not flash must keep one style:\n%q", out)
	}
	if strings.Index(out, "hello") > strings.Index(out, "done") {
		t.Errorf("output out of order:\n%q", out)
	}
	if strings.Index(out, "\x1b[r") < strings.LastIndex(out, "done") {
		t.Errorf("the screen must be given back only after the shell's output:\n%q", out)
	}
}

func TestRunFlashesTheBanner(t *testing.T) {
	ft := newFakeTerminal(t, 80, 24)
	cmd := exec.Command("sh", "-c", "sleep 1.3")
	if err := Run(cmd, ft.tty, ft.tty, Banner{Text: "PROD demo", Flash: true}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := ft.finish()
	if !strings.Contains(out, styleAlert) || !strings.Contains(out, styleWarn) {
		t.Errorf("a flashing banner should show both styles:\n%q", out)
	}
	if strings.Index(out, "\x1b[r") < strings.LastIndex(out, styleWarn) {
		t.Errorf("no repaint may follow the screen being given back:\n%q", out)
	}
}

func TestRunFollowsResizes(t *testing.T) {
	ft := newFakeTerminal(t, 80, 24)
	cmd := exec.Command("sh", "-c", "sleep 1; stty size")
	errc := make(chan error, 1)
	go func() { errc <- Run(cmd, ft.tty, ft.tty, Banner{Text: "PROD demo"}) }()

	ft.waitFor(t, "\x1b[1;23r") // the session is up
	if err := pty.Setsize(ft.master, &pty.Winsize{Rows: 30, Cols: 100}); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGWINCH); err != nil {
		t.Fatal(err)
	}

	if err := <-errc; err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := ft.finish()
	if !strings.Contains(out, "\x1b[1;29r") {
		t.Errorf("the region should follow the new size:\n%q", out)
	}
	if !strings.Contains(out, "29 100") {
		t.Errorf("the shell should see one row less than the new size:\n%q", out)
	}
}

func TestRunWithoutRoomForABanner(t *testing.T) {
	ft := newFakeTerminal(t, 80, 2)
	cmd := exec.Command("sh", "-c", "echo hi")
	if err := Run(cmd, ft.tty, ft.tty, Banner{Text: "PROD demo"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := ft.finish()
	if !strings.Contains(out, "hi") || strings.Contains(out, "PROD demo") || strings.Contains(out, "\x1b[1;") {
		t.Errorf("a tiny terminal should run the shell plainly:\n%q", out)
	}
}

func TestScreenOpenVacatesTheBannerRow(t *testing.T) {
	var out bytes.Buffer
	s := newScreen(&out, 10, 5, Banner{Text: "PROD"})
	s.open(5) // cursor on the last row: scroll first, then start on row 4
	if got := out.String(); !strings.HasPrefix(got, "\n") || !strings.HasSuffix(got, "\x1b[4;1H") {
		t.Errorf("open(5) wrote %q", got)
	}
	out.Reset()
	s.open(2)
	if got := out.String(); strings.HasPrefix(got, "\n") || !strings.HasSuffix(got, "\x1b[2;1H") {
		t.Errorf("open(2) wrote %q", got)
	}
}

func TestScreenLine(t *testing.T) {
	s := newScreen(&bytes.Buffer{}, 10, 5, Banner{Text: "PROD", Flash: true})
	if got, want := s.line(), styleAlert+"   PROD   \x1b[0m"; got != want {
		t.Errorf("centered line = %q, want %q", got, want)
	}
	s.phase = 1
	if got := s.line(); !strings.HasPrefix(got, styleWarn) {
		t.Errorf("the other phase should use the warn style: %q", got)
	}

	s = newScreen(&bytes.Buffer{}, 12, 5, Banner{Text: "a very long banner text", Short: "short"})
	if got := s.line(); !strings.Contains(got, "   short    ") {
		t.Errorf("the short text should be used when the long one does not fit: %q", got)
	}
	s = newScreen(&bytes.Buffer{}, 4, 5, Banner{Text: "truncated", Short: "trimmed"})
	if got := s.line(); !strings.Contains(got, "trim") || strings.Contains(got, "trimm") {
		t.Errorf("text that never fits is cut to the width: %q", got)
	}
}
