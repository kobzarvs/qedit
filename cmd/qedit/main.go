package main

import (
	"fmt"
	"os"

	"github.com/kobzarvs/qedit/internal/app"
)

func main() {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if err := app.New(args).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "qedit:", err)
		os.Exit(1)
	}
}
