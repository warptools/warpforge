package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/ipld/go-ipld-prime"
	ipldjson "github.com/ipld/go-ipld-prime/codec/json"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/printer"
	"github.com/urfave/cli/v2"
)

const VERSION = "0.0.1"

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	app := cli.NewApp()
	app.Name = "warpforge"
	app.Version = VERSION
	app.Usage = "Putting things together. Consistently."
	app.Writer = stdout
	app.ErrWriter = stderr
	app.Reader = stdin
	cli.VersionFlag = &cli.BoolFlag{
		Name: "version",
	}
	app.HideVersion = true
	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
		},
		&cli.BoolFlag{
			Name: "json",
		},
	}
	app.ExitErrHandler = exitErrHandler
	app.After = afterFunc
	app.Commands = []*cli.Command{
		{
			Name:  "plot",
			Usage: "Subcommands that operate on plots",
			Subcommands: []*cli.Command{
				{
					Name:   "graph",
					Usage:  "Generate a graph from a plot file",
					Action: cmdPlotGraph,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:  "png",
							Usage: "Output graph PNG to `FILE`",
						},
					},
				},
			},
		},
		{
			Name:   "run",
			Usage:  "Run a module or formula",
			Action: cmdRun,
		},
		{
			Name:   "check",
			Usage:  "Check an input file for syntax and sanity",
			Action: cmdCheck,
		},
	}
	err := app.Run(args)
	return 0, err
}

// Called after a command returns an non-nil error value.
// Prints the formatted error to stderr.
func exitErrHandler(c *cli.Context, err error) {
	if c.Bool("json") {
		bytes, err := json.Marshal(err)
		if err != nil {
			panic("error marshaling json")
		}
		fmt.Fprintf(c.App.ErrWriter, "%s\n", string(bytes))
	} else {
		fmt.Fprintf(c.App.ErrWriter, "error: %s\n", err)
	}
}

// Called after any command completes. The comamnd may optionally set
// c.App.Metadata["result"] to a datamodel.Node value before returning to
// have the result output to stdout.
func afterFunc(c *cli.Context) error {
	// if a Node named "result" exists in the metadata,
	// print it to stdout in the desired format
	if c.App.Metadata["result"] != nil {
		n, ok := c.App.Metadata["result"].(datamodel.Node)
		if !ok {
			panic("invalid result value - not a datamodel.Node")
		}

		if c.Bool("json") {
			serial, err := ipld.Encode(n, ipldjson.Encode)
			if err != nil {
				panic("failed to serialize output")
			}
			fmt.Fprintf(c.App.Writer, "%s\n", serial)
		} else {
			fmt.Fprintf(c.App.Writer, "ok: %s\n", printer.Sprint(n))
		}
	}
	return nil
}

func main() {
	exitCode, _ := Run(os.Args, os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}
