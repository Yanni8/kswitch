package main

import (
	"fmt"
	"os"

	"github.com/Yanni8/kswitch/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "kswitch: %v\n", err)
		os.Exit(1)
	}
}
