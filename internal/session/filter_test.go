// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"strings"
	"testing"
)

func newTestFilter() *filter {
	return &filter{rows: 24, repaint: "<paint>"}
}

func TestFilterPassesOrdinaryOutput(t *testing.T) {
	in := "hello\r\n\x1b[1mbold\x1b[0m \x1b]0;title\x07 \x1b(B \x1b[?25l \x1b7x\x1b8"
	if got := string(newTestFilter().process([]byte(in))); got != in {
		t.Errorf("ordinary output was changed:\n got %q\nwant %q", got, in)
	}
}

func TestFilterConfinesScrollRegion(t *testing.T) {
	tests := map[string]string{
		"\x1b[r":     "\x1b[1;23r",
		"\x1b[0;0r":  "\x1b[1;23r",
		"\x1b[1;24r": "\x1b[1;23r",
		"\x1b[5;40r": "\x1b[5;23r",
		"\x1b[1;10r": "\x1b[1;10r",
		"\x1b[23;5r": "\x1b[1;23r", // nonsense falls back to the whole region
	}
	for in, want := range tests {
		if got := string(newTestFilter().process([]byte(in))); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}

func TestFilterRepaintsAfterDisturbances(t *testing.T) {
	for _, seq := range []string{"\x1b[2J", "\x1b[3J", "\x1b[J", "\x1b[0J", "\x1b[?1049h", "\x1b[?1049l", "\x1b[?47h", "\x1b[?1047l", "\x1bc"} {
		if got := string(newTestFilter().process([]byte(seq))); got != seq+"<paint>" {
			t.Errorf("%q -> %q, want a repaint after it", seq, got)
		}
	}
	for _, seq := range []string{"\x1b[1J", "\x1b[K", "\x1b[2K", "\x1b[?25l", "\x1b[?2004h", "\x1b[H"} {
		if got := string(newTestFilter().process([]byte(seq))); got != seq {
			t.Errorf("%q -> %q, want it untouched", seq, got)
		}
	}
}

func TestFilterJoinsSequencesSplitAcrossReads(t *testing.T) {
	f := newTestFilter()
	if got := string(f.process([]byte("abc\x1b["))); got != "abc" {
		t.Errorf("first part: got %q, want the unfinished sequence held back", got)
	}
	if got := string(f.process([]byte("2Jdef"))); got != "\x1b[2J<paint>def" {
		t.Errorf("second part: got %q", got)
	}

	f.process([]byte("\x1b]0;ti"))
	if got := string(f.process([]byte("tle\x07x"))); got != "\x1b]0;title\x07x" {
		t.Errorf("split OSC: got %q", got)
	}

	if got := string(f.process([]byte("\x1b"))); got != "" {
		t.Errorf("lone ESC should be held back, got %q", got)
	}
	if got := string(f.flush()); got != "\x1b" {
		t.Errorf("flush: got %q", got)
	}
}

func TestFilterGivesUpOnEndlessSequences(t *testing.T) {
	long := "\x1b]" + strings.Repeat("x", maxSequence)
	if got := string(newTestFilter().process([]byte(long))); got != long {
		t.Error("an overlong unfinished sequence must be passed on")
	}
}
