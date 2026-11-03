package cli

import (
	"os"
	"os/exec"
	"strings"
)

// openEditor launches the user's editor on path and blocks until it
// exits. It is a variable so tests can substitute a fake editor.
var openEditor = func(path string) error {
	args := append(editorCommand(), path)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// editorCommand resolves the editor from $VISUAL, then $EDITOR, falling
// back to vim. The value may carry arguments (e.g. "code --wait").
func editorCommand() []string {
	for _, env := range []string{"VISUAL", "EDITOR"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return strings.Fields(v)
		}
	}
	return []string{"vim"}
}
