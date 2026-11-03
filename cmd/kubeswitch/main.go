package main

import (
	"fmt"
	"os"

	"github.com/Yanni8/kubeswitch/internal/cli"
)

func main() {
	err := cli.Execute()
	if err == nil {
		return
	}
	// A shell started by "kubeswitch use" passes its exit status through.
	if code, ok := cli.ExitStatus(err); ok {
		os.Exit(code)
	}
	fmt.Fprintf(os.Stderr, "kubeswitch: %v\n", err)
	os.Exit(1)
}
