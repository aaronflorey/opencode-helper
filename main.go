package main

import (
	"fmt"
	"os"

	"opencode-cli/internal/cli"
)

func main() {
	root := cli.NewRootCommand()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
