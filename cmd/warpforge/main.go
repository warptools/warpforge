package main

import (
	"os"

	wfapp "github.com/warptools/warpforge/app"
)

func main() {
	wfapp.App.Reader = os.Stdin
	wfapp.App.Writer = os.Stdout
	wfapp.App.ErrWriter = os.Stderr
	err := wfapp.App.Run(os.Args)
	if err != nil {
		os.Exit(1) // TODO use richer exit codes
	}
}
