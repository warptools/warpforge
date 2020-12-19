package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	cli "github.com/urfave/cli/v2"
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

	Beware there's no readline-like features such as up-arrow-for-last-command available in interactive mode.
	There is also no support for customization of the prompt via "PS*" variables.
	Sorry.

	// the following is still TODO:

	Some flags can change behavior modes (dramatically).

		--interactiveerror     # this tells smsh to stay open and switch to interactive mode instead of exiting in case of an execution error.
		--control=/dev/fd/3    # this tells smsh to expect a controller to speak to it over smsh's fd 3.
		--snippet=/do/this     # this tells smsh to read a snippet from a file.  Can be used repeatedly.

	Use "--" to signal the end of flags; all following arguments will be processed as a script snippet each.

*/

func main() {
	app := &cli.App{
		Name: "smsh",
		Action: func(args *cli.Context) error {
			snippets := make([]snippet, args.Args().Len())
			for _, arg := range args.Args().Slice() {
				snippets = append(snippets, snippet{body: arg})
			}
			if err := parseAll(snippets); err != nil {
				return cli.Exit(fmt.Errorf("snippets didn't parse: %v\n", err), 4)
			}
			if err := runAll(snippets); err != nil {
				return cli.Exit(fmt.Errorf("%s\n", err.Error()), 9)
			}
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "smsh: error: %s\n", err)
	}

	//snippets := []snippet{
	//	{body: "ls"},
	//	{body: "ps"},
	//	{body: "unknowncommand; echo no problem"},
	//	{body: "unknowncommand; isaproblem"},
	//	{body: "echo unreached"},
	//	//{body: "(no parse"},
	//}
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
		//r.Reset() // don't, actually.  we want variables and stuff to carry between scriptlets.
		ctx := context.Background()
		err := r.Run(ctx, snip.parsed)
		if err != nil {
			err = fmt.Errorf("smsh: error while processing %d'th command (%q): %w", i, snip.body, err)
			// fixme: figure out how to abstract this properly so this error doesn't need handling in weirdly divergent ways depending on mode.
			// fixme: actually get the critical mode flag down here
			if true {
				return runInteractive(r, os.Stdin, os.Stdout, os.Stderr)
			}
		}
	}
	return nil
}

func runInteractive(r *interp.Runner, stdin io.Reader, stdout, stderr io.Writer) error {
	parser := syntax.NewParser()
	fmt.Fprintf(stdout, "$ ")
	var runErr error
	fn := func(stmts []*syntax.Stmt) bool {
		if parser.Incomplete() {
			fmt.Fprintf(stdout, "> ")
			return true
		}
		ctx := context.Background()
		for _, stmt := range stmts {
			runErr = r.Run(ctx, stmt)
			if r.Exited() {
				return false
			}
		}
		fmt.Fprintf(stdout, "$ ")
		return true
	}
	if err := parser.Interactive(stdin, fn); err != nil {
		return err
	}
	return runErr
}
