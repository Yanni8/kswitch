// Copyright 2026 Yannick Müller
// SPDX-License-Identifier: Apache-2.0

// Package prompt implements the questions kubeswitch asks interactively:
// arrow-key menus and an inline text field when attached to a terminal,
// plain line-based prompts otherwise (piped input, tests).
package prompt

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// ErrAborted is returned when the user cancels a prompt (Ctrl-C, Esc, q)
// or when input ends.
var ErrAborted = errors.New("aborted")

// Option is one entry of a Select menu.
type Option struct {
	Value string // returned when chosen; also accepted as a typed answer
	Label string // shown to the user
}

// InputOptions configures Input.
type InputOptions struct {
	// Placeholder is suggested while the field is empty: it is shown
	// dimmed, accepted by Enter, and copied into the field by Tab or →.
	Placeholder string
	// Validate, if set, checks the answer. A non-empty problem is shown
	// to the user, who is asked again; an error aborts the prompt.
	Validate func(answer string) (problem string, err error)
}

// Prompter asks questions on an input/output pair.
type Prompter struct {
	in  *bufio.Reader
	out io.Writer
	tty *os.File // the input terminal, nil when prompts can't be interactive
}

// New returns a Prompter reading answers from in and writing to out.
// Prompts are interactive only when both are terminals.
func New(in io.Reader, out io.Writer) *Prompter {
	p := &Prompter{in: bufio.NewReader(in), out: out}
	if IsTerminal(in) && IsTerminal(out) && os.Getenv("TERM") != "dumb" {
		p.tty = in.(*os.File)
	}
	return p
}

// IsTerminal reports whether v is an *os.File attached to a terminal.
func IsTerminal(v any) bool {
	f, ok := v.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// ReadLine prints label and returns the next line of input, trimmed of
// surrounding whitespace. When input ends, whatever was read is returned
// together with io.EOF.
func (p *Prompter) ReadLine(label string) (string, error) {
	fmt.Fprint(p.out, label)
	line, err := p.in.ReadString('\n')
	return strings.TrimSpace(line), err
}

// Select asks the user to pick one of opts and returns its Value.
func (p *Prompter) Select(title string, opts []Option) (string, error) {
	return p.SelectDefault(title, opts, "")
}

// SelectDefault is Select with the cursor starting on the option whose
// Value is def; without a terminal, an empty answer picks that option.
// An unknown def is ignored.
func (p *Prompter) SelectDefault(title string, opts []Option, def string) (string, error) {
	initial := 0
	found := false
	for i, o := range opts {
		if o.Value == def {
			initial, found = i, true
			break
		}
	}
	if !found {
		def = ""
	}
	return p.ask(
		func() (string, error) { return p.selectNumbered(title, opts, def) },
		func(width int) (string, error) { return selectMenu(p.in, p.out, width, title, opts, initial) },
	)
}

// Input asks for a line of text.
func (p *Prompter) Input(title string, o InputOptions) (string, error) {
	return p.ask(
		func() (string, error) { return p.inputLine(title, o) },
		func(width int) (string, error) { return inputField(p.in, p.out, width, title, o) },
	)
}

// ask runs interactive on the terminal in raw mode, or plain when there
// is no terminal to take over.
func (p *Prompter) ask(plain func() (string, error), interactive func(width int) (string, error)) (string, error) {
	if p.tty == nil {
		return plain()
	}
	fd := int(p.tty.Fd())
	saved, err := term.MakeRaw(fd)
	if err != nil {
		return plain()
	}
	defer term.Restore(fd, saved)

	width, _, err := term.GetSize(fd)
	if err != nil || width <= 0 {
		width = 80
	}
	return interactive(width)
}

// selectNumbered is the non-interactive Select: a numbered list that
// accepts either a number or an option's value, or nothing when there
// is a default.
func (p *Prompter) selectNumbered(title string, opts []Option, def string) (string, error) {
	fmt.Fprintln(p.out, title)
	for i, o := range opts {
		fmt.Fprintf(p.out, "  %d) %s\n", i+1, o.Label)
	}
	label := fmt.Sprintf("Select [1-%d]: ", len(opts))
	if def != "" {
		label = fmt.Sprintf("Select [1-%d] (default: %s): ", len(opts), def)
	}
	for {
		answer, readErr := p.ReadLine(label)
		answer = strings.ToLower(answer)

		if answer == "" && def != "" {
			return def, nil
		}
		if n, err := strconv.Atoi(answer); err == nil && n >= 1 && n <= len(opts) {
			return opts[n-1].Value, nil
		}
		for _, o := range opts {
			if answer == strings.ToLower(o.Value) {
				return o.Value, nil
			}
		}
		if readErr != nil {
			return "", ErrAborted
		}
		fmt.Fprintf(p.out, "Please answer with 1-%d or one of the names listed.\n", len(opts))
	}
}

// inputLine is the non-interactive Input: "title [placeholder]: ".
func (p *Prompter) inputLine(title string, o InputOptions) (string, error) {
	label := title + ": "
	if o.Placeholder != "" {
		label = fmt.Sprintf("%s [%s]: ", title, o.Placeholder)
	}
	for {
		answer, readErr := p.ReadLine(label)
		if answer == "" {
			answer = o.Placeholder
		}
		problem, err := o.validate(answer)
		if err != nil {
			return "", err
		}
		if problem == "" {
			return answer, nil
		}
		if readErr != nil {
			return "", ErrAborted
		}
		fmt.Fprintln(p.out, problem)
	}
}

func (o InputOptions) validate(answer string) (string, error) {
	if o.Validate == nil {
		return "", nil
	}
	return o.Validate(answer)
}
