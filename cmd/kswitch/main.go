package main

import (
	"fmt"
	"os"

	"github.com/Yanni8/kswitch/internal/cli"
)

func main() {
	err := cli.Execute()
	if err == nil {
		return
	}
	// A shell started by "kswitch use" passes its exit status through.
	if code, ok := cli.ExitStatus(err); ok {
		os.Exit(code)
	}
	fmt.Fprintf(os.Stderr, "kswitch: %v\n", err)
	os.Exit(1)
}
