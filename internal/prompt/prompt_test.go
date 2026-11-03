package prompt

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

var testOpts = []Option{
	{Value: "prod", Label: "prod"},
	{Value: "test", Label: "test"},
	{Value: "x", Label: "X marks the spot"},
}

// runMenu drives the arrow-key menu with the given raw key bytes.
func runMenu(t *testing.T, keys string) (value, output string, err error) {
	t.Helper()
	var out bytes.Buffer
	value, err = selectMenu(bufio.NewReader(strings.NewReader(keys)), &out, 80, "Pick:", testOpts)
	return value, out.String(), err
}

func TestMenuKeys(t *testing.T) {
	tests := []struct{ name, keys, want string }{
		{"enter picks the first", "\r", "prod"},
		{"down arrow", "\x1b[B\r", "test"},
		{"up arrow", "\x1b[B\x1b[B\x1b[A\r", "test"},
		{"application-mode arrows", "\x1bOB\x1bOB\x1bOA\r", "test"},
		{"vim keys", "jjk\r", "test"},
		{"newline confirms too", "j\n", "test"},
		{"clamps at both ends", "kk\x1b[B\x1b[B\x1b[B\x1b[B\r", "x"},
		{"unknown keys are ignored", "zz\x1b[C\x1b[3~\x1b[B\r", "test"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := runMenu(t, tt.keys)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMenuOutput(t *testing.T) {
	_, out, err := runMenu(t, "\x1b[B\r")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, hideCursor) || !strings.HasSuffix(out, showCursor) {
		t.Errorf("cursor should be hidden during the menu and shown afterwards:\n%q", out)
	}
	for _, want := range []string{bold("> prod") + "\r\n", bold("> test") + "\r\n", "Pick: test\r\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%q", want, out)
		}
	}
}

func TestMenuAborts(t *testing.T) {
	for name, keys := range map[string]string{
		"ctrl-c":   "\x03",
		"ctrl-d":   "\x04",
		"q":        "q",
		"lone esc": "\x1b",
		"eof":      "jj",
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := runMenu(t, keys)
			if !errors.Is(err, ErrAborted) {
				t.Errorf("got %v, want ErrAborted", err)
			}
		})
	}
}

// runField drives the inline text field with the given raw key bytes.
func runField(t *testing.T, keys string, o InputOptions) (value, output string, err error) {
	t.Helper()
	var out bytes.Buffer
	value, err = inputField(bufio.NewReader(strings.NewReader(keys)), &out, 80, "Name", o)
	return value, out.String(), err
}

func TestFieldKeys(t *testing.T) {
	sugg := InputOptions{Placeholder: "sugg"}
	tests := []struct {
		name string
		keys string
		opts InputOptions
		want string
	}{
		{"enter accepts the placeholder", "\r", sugg, "sugg"},
		{"typed text replaces it", "Pay\r", sugg, "Pay"},
		{"answers are trimmed", "  Pay \n", sugg, "Pay"},
		{"tab edits the placeholder", "\t-2\r", sugg, "sugg-2"},
		{"right arrow edits the placeholder", "\x1b[C!\r", sugg, "sugg!"},
		{"tab does nothing once typing", "a\tb\r", sugg, "ab"},
		{"backspace", "abc\x7f\x7f\r", sugg, "a"},
		{"backspace restores the placeholder", "a\x7f\r", sugg, "sugg"},
		{"ctrl-u clears", "abc\x15x\r", sugg, "x"},
		{"ctrl-w deletes a word", "foo bar  \x17baz\r", sugg, "foo baz"},
		{"unicode", "Zürich\r", sugg, "Zürich"},
		{"other sequences are ignored", "a\x1b[3~\x1b[D\x1bOAb\r", sugg, "ab"},
		{"no placeholder", "\r", InputOptions{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := runField(t, tt.keys, tt.opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFieldOutput(t *testing.T) {
	_, out, err := runField(t, "P\r", InputOptions{Placeholder: "sugg"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"> " + dim("sugg"), // placeholder shown while empty
		dim("Enter to accept the suggestion, Tab to edit it"), // and its hint
		"> P\r\n" + dim("Enter to confirm"),                   // placeholder gone after the first key
		"Name: P\r\n",                                         // collapsed summary
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%q", want, out)
		}
	}
	if strings.Contains(out, hideCursor) {
		t.Error("the cursor must stay visible in a text field")
	}
}

func TestFieldValidation(t *testing.T) {
	o := InputOptions{
		Placeholder: "bad",
		Validate: func(s string) (string, error) {
			if s == "bad" {
				return "Nope.", nil
			}
			return "", nil
		},
	}
	// Enter accepts "bad" -> rejected; typing clears the problem; "good" passes.
	got, out, err := runField(t, "\rgood\r", o)
	if err != nil || got != "good" {
		t.Fatalf("got %q, %v; want %q", got, err, "good")
	}
	if !strings.Contains(out, red("Nope.")) {
		t.Errorf("problem not shown:\n%q", out)
	}

	boom := errors.New("boom")
	_, _, err = runField(t, "\r", InputOptions{Validate: func(string) (string, error) { return "", boom }})
	if !errors.Is(err, boom) {
		t.Errorf("validator error not returned: %v", err)
	}
}

func TestFieldAborts(t *testing.T) {
	for name, keys := range map[string]string{
		"ctrl-c":   "ab\x03",
		"ctrl-d":   "\x04",
		"lone esc": "ab\x1b",
		"eof":      "ab",
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := runField(t, keys, InputOptions{Placeholder: "sugg"})
			if !errors.Is(err, ErrAborted) {
				t.Errorf("got %v, want ErrAborted", err)
			}
		})
	}
}

func TestFitPlaceholder(t *testing.T) {
	tests := []struct {
		in   string
		max  int
		want string
	}{
		{"abc", 7, "abc"},
		{"abcdefg", 7, "abcdefg"},
		{"abcdefghijkl", 7, "abcdef…"},
		{"abc", 0, ""},
	}
	for _, tt := range tests {
		if got := fitPlaceholder(tt.in, tt.max); got != tt.want {
			t.Errorf("fitPlaceholder(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
		}
	}
}

func TestScreenDrawHandlesWrappedLines(t *testing.T) {
	var out bytes.Buffer
	s := &screen{out: &out, width: 10}

	// The middle line is 14 columns wide, so it wraps onto a second row.
	s.draw([]string{"T", "> abcdefghijkl", "hint"}, 1, 14)
	want := "\r\x1b[J" + "T\r\n> abcdefghijkl\r\nhint" +
		"\x1b[1A" + "\r" + "\x1b[4C" // up to the wrapped row, column 4
	if out.String() != want {
		t.Errorf("draw wrote %q, want %q", out.String(), want)
	}
	if s.parked != 2 {
		t.Errorf("parked = %d, want 2", s.parked)
	}

	out.Reset()
	s.clear()
	if out.String() != "\x1b[2A\r\x1b[J" {
		t.Errorf("clear wrote %q", out.String())
	}

	// A line that exactly fills the width stays on one row, cursor at
	// the last column.
	out.Reset()
	s.draw([]string{"> 12345678"}, 0, 10)
	if want := "\r\x1b[J> 12345678\r\x1b[10C"; out.String() != want {
		t.Errorf("draw wrote %q, want %q", out.String(), want)
	}
}

func TestVisibleWidth(t *testing.T) {
	tests := map[string]int{
		"":                       0,
		"abc":                    3,
		bold("> x"):              3,
		dim("↑/↓"):               3,
		"\x1b[41m      \x1b[0m":  6,
		"\x1bOA":                 0,
		"a\x1bb":                 2,
		"Zürich":                 6,
		"a\x1b[1;5Cb":            2,
		"trailing escape \x1b[1": 16,
	}
	for in, want := range tests {
		if got := visibleWidth(in); got != want {
			t.Errorf("visibleWidth(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestSelectFallsBackToNumberedList(t *testing.T) {
	var out bytes.Buffer
	p := New(strings.NewReader("banana\n2\n"), &out)
	if p.tty != nil {
		t.Fatal("a strings.Reader must not count as a terminal")
	}

	got, err := p.Select("Pick:", testOpts)
	if err != nil {
		t.Fatal(err)
	}
	if got != "test" {
		t.Errorf("got %q, want %q", got, "test")
	}
	for _, want := range []string{"Pick:\n", "  1) prod\n", "  3) X marks the spot\n", "Please answer with 1-3"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "\x1b[") {
		t.Error("fallback mode must not emit terminal control sequences")
	}
}

func TestSelectNumberedAcceptsValues(t *testing.T) {
	p := New(strings.NewReader("X\n"), io.Discard)
	if got, err := p.Select("Pick:", testOpts); err != nil || got != "x" {
		t.Errorf("got %q, %v; want %q", got, err, "x")
	}
}

func TestSelectNumberedAbortsAtEOF(t *testing.T) {
	for _, in := range []string{"", "9", "9\n"} {
		p := New(strings.NewReader(in), io.Discard)
		if _, err := p.Select("Pick:", testOpts); !errors.Is(err, ErrAborted) {
			t.Errorf("input %q: got %v, want ErrAborted", in, err)
		}
	}
}

func TestInputFallsBackToLine(t *testing.T) {
	var out bytes.Buffer
	p := New(strings.NewReader("\n"), &out)
	got, err := p.Input("Name", InputOptions{Placeholder: "sugg"})
	if err != nil || got != "sugg" {
		t.Errorf("got %q, %v; want the placeholder", got, err)
	}
	if out.String() != "Name [sugg]: " {
		t.Errorf("label = %q", out.String())
	}

	out.Reset()
	p = New(strings.NewReader("  Custom \n"), &out)
	if got, err := p.Input("Name", InputOptions{}); err != nil || got != "Custom" {
		t.Errorf("got %q, %v; want %q", got, err, "Custom")
	}
	if out.String() != "Name: " {
		t.Errorf("label = %q", out.String())
	}
}

func TestInputLineValidation(t *testing.T) {
	var out bytes.Buffer
	o := InputOptions{Validate: func(s string) (string, error) {
		if s == "" {
			return "A name is required.", nil
		}
		return "", nil
	}}
	p := New(strings.NewReader("\nfine\n"), &out)
	if got, err := p.Input("Name", o); err != nil || got != "fine" {
		t.Errorf("got %q, %v; want %q", got, err, "fine")
	}
	if !strings.Contains(out.String(), "A name is required.\n") {
		t.Errorf("problem not shown:\n%q", out.String())
	}

	p = New(strings.NewReader(""), io.Discard)
	if _, err := p.Input("Name", o); !errors.Is(err, ErrAborted) {
		t.Errorf("got %v, want ErrAborted at EOF", err)
	}
}

func TestReadLine(t *testing.T) {
	var out bytes.Buffer
	p := New(strings.NewReader("  hello \nrest"), &out)

	line, err := p.ReadLine("Name: ")
	if err != nil || line != "hello" {
		t.Errorf("got %q, %v; want %q", line, err, "hello")
	}
	if out.String() != "Name: " {
		t.Errorf("label not printed: %q", out.String())
	}

	line, err = p.ReadLine("Again: ")
	if !errors.Is(err, io.EOF) || line != "rest" {
		t.Errorf("got %q, %v; want %q with io.EOF", line, err, "rest")
	}
}
