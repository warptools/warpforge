package main

import (
	"context"
	"fmt"
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
	(All parsing is done before execution begins.  Nothing will be executed if _any_ snippets have a parse error.)
	Each snippet will be executed in sequence.
	Any snippet exiting non-zero will cause work to stop; no subsequent snippets will be evaluated.

	Within each snippet, the rules are bash-like.
	Errors in the middle of a snippet do not cause evaluation to halt unless you've applied "set -e" mode
	(despite the fast-exit behavior _between_ each snippet).
	Not all bash features are supported, but most are: see https://github.com/mvdan/sh for details.

	A short human-readable header will be inserted in the output stream (on stderr) before each script snippet is evaluated.
	(This should not be relied on for parsing, since it mangles together the control plane and data plane;
	use other mode arguments to get safe machine-processable information.)

	Anything smsh says will be on stderr (as opposed to stdout),
	so you can run sequences of commands in smsh and still pipe the output and expect it to be processable normally
	(presuming the commands you're running create processable stdout streams in the first place, anyway).

	// the following is still TODO:

	Some flags can change behavior modes (dramatically).

		--control=/dev/fd/3    # this tells smsh to expect a controller to speak to it over smsh's fd 3.
		--snippet=/do/this     # this tells smsh to read a snippet from a file.  Can be used repeatedly.

	Use "--" to signal the end of flags; all following arguments will be processed as a script snippet each.

*/

func main() {
	snippets := []snippet{
		{body: "ls"},
		{body: "ps"},
		{body: "unknowncommand; echo no problem"},
		{body: "unknowncommand; isaproblem"},
		{body: "echo unreached"},
		//{body: "(no parse"},
	}
	if err := parseAll(snippets); err != nil {
		fmt.Fprintf(os.Stderr, "smsh: snippets didn't parse: %v\n", err)
		os.Exit(4)
	}
	if err := runAll(snippets); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(9)
	}
}

type snippet struct {
	name   string
	body   string
	parsed *syntax.File
}

func (snip *snippet) Parse() (err error) {
	snip.parsed, err = syntax.NewParser().Parse(strings.NewReader(snip.body), snip.name)
	return
}

func parseAll(snippets []snippet) error {
	// todo: i'd love for this to return an aggregated list of errors, rather than just the first one.  does the golang community have a widely accepted standard for that yet?
	for i := range snippets {
		if err := snippets[i].Parse(); err != nil {
			return err
		}
	}
	return nil
}

func runAll(snippets []snippet) error {
	r, err := interp.New(interp.StdIO(os.Stdin, os.Stdout, os.Stderr))
	if err != nil {
		return fmt.Errorf("smsh: failed to initialize interpreter: %w", err)
	}
	for i, snip := range snippets {
		fmt.Fprintf(os.Stderr, "\033[35m>>smsh>script%d>>\033[0m\n", i)
		r.Reset()
		ctx := context.Background()
		err := r.Run(ctx, snip.parsed)
		if err != nil {
			return fmt.Errorf("smsh: error while processing %d'th command (%q): %w", i, snip.body, err)
		}
	}
	return nil
}
