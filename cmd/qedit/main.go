package main

import (
	"fmt"
	"os"

	"github.com/kobzarvs/qedit/internal/app"
	"github.com/kobzarvs/qedit/internal/logger"
)

func main() {
	// Initialize logger (debug mode if QEDIT_DEBUG is set)
	debug := os.Getenv("QEDIT_DEBUG") != ""
	if err := logger.Init(debug); err != nil {
		fmt.Fprintln(os.Stderr, "qedit: failed to init logger:", err)
	}
	defer logger.Close()

	logger.Info("qedit starting", "args", os.Args[1:], "debug", debug)

	args := os.Args[1:]
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if err := app.New(args).Run(); err != nil {
		logger.Error("qedit exited with error", "error", err)
		fmt.Fprintln(os.Stderr, "qedit:", err)
		os.Exit(1)
	}
	logger.Info("qedit exited normally")
}
