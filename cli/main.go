package main

import (
	"os"

	"github.com/bold-minds/trek/cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
