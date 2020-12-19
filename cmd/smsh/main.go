package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

/*
	smsh is something between a shell and a process monitor.
	Or rather: it's entirely a shell, but it also comes with some command-and-control introspection features
	that are very useful if you want to wrap other tools around it and drive things in a controlled way.

	smsh executes a series of script "snippets" of a bash-like syntax.
	Each snippet must parse cleanly on its own.
	Each snippet will be executed in sequence.
	Any snippet exiting non-zero will cause work to stop; no subsequent snippets will be evaluated.

	Within each snippet, the rules are bash-like.
	Errors in the middle of a snippet do not cause evaluation to halt unless you've applied "set -e" mode
	(despite the fast-exit behavior _between_ each snippet).
	Not all bash features are supported, but most are: see https://github.com/mvdan/sh for details.

	// the following is still TODO:

	Some flags can change behavior modes (dramatically).

		--control=/dev/fd/3    # this tells smsh to expect a controller to speak to it over smsh's fd 3.
		--snippet=/do/this     # this tells smsh to read a snippet from a file.  Can be used repeatedly.

	Use "--" to signal the end of flags; all following arguments will be processed as a script snippet each.

*/

func main() {
	snippets := []string{
		"ls",
		"ps",
		"unknowncommand; echo no problem",
		"unknowncommand; isaproblem",
		"echo unreached",
	}
	runAll(snippets)
}

func runAll(snippets []string) error {
	r, err := interp.New(interp.StdIO(os.Stdin, os.Stdout, os.Stderr))
	if err != nil {
		return fmt.Errorf("smsh: failed to initialize interpreter: %w", err)
	}
	for i, snip := range snippets {
		fmt.Fprintf(os.Stdout, "smsh>cmd%d>>\n", i)
		err := run(r, strings.NewReader(snip), fmt.Sprintf("cmd%d", i))
		if err != nil {
			return fmt.Errorf("smsh: error while processing %d'th command (%q): %w", i, snip, err)
		}
	}
	return nil
}

func run(r *interp.Runner, reader io.Reader, name string) error {
	prog, err := syntax.NewParser().Parse(reader, name)
	if err != nil {
		return err
	}
	r.Reset()
	ctx := context.Background()
	return r.Run(ctx, prog)
}