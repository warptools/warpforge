package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/ipld/go-ipld-prime"
	ipldjson "github.com/ipld/go-ipld-prime/codec/json"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/urfave/cli/v2"
)

const VERSION = "v0.4.0"

func makeApp(stdin io.Reader, stdout, stderr io.Writer) *cli.App {
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
			Name: "quiet",
		},
		&cli.BoolFlag{
			Name:  "json",
			Usage: "Enable JSON API output",
		},
		&cli.StringFlag{
			Name:      "trace.file",
			Usage:     "Enable tracing and emit output to file",
			TakesFile: true,
		},
		&cli.BoolFlag{
			Name:   "trace.grpc.enable",
			Usage:  "Enable remote tracing",
			Hidden: true, // not implemented yet
		},
		&cli.StringFlag{
			Name:   "trace.grpc.endpoint",
			Usage:  "Sets an endpoint for remote open-telemetry tracing collection",
			Hidden: true, // not implemented yet
		},
		&cli.BoolFlag{
			Name:  "trace.http.enable",
			Usage: "Enable remote tracing over http",
		},
		&cli.BoolFlag{
			Name:  "trace.http.insecure",
			Usage: "Allows insecure http",
		},
		&cli.StringFlag{
			Name:  "trace.http.endpoint",
			Usage: "Sets an endpoint for remote open-telemetry tracing collection",
		},
	}
	app.ExitErrHandler = exitErrHandler
	app.After = afterFunc
	app.Commands = []*cli.Command{
		&runCmdDef,
		&checkCmdDef,
		&catalogCmdDef,
		&watchCmdDef,
		&statusCmdDef,
		&quickstartCmdDef,
		&ferkCmdDef,
		&healthCmdDef,
	}
	return app
}

// Called after a command returns an non-nil error value.
// Prints the formatted error to stderr.
func exitErrHandler(c *cli.Context, err error) {
	if err == nil {
		return
	}
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

		serial, err := ipld.Encode(n, ipldjson.Encode)
		if err != nil {
			panic("failed to serialize output")
		}
		fmt.Fprintf(c.App.Writer, "%s\n", serial)
	}
	return nil
}

func main() {
	err := makeApp(os.Stdin, os.Stdout, os.Stderr).Run(os.Args)
	if err != nil {
		os.Exit(1)
	}
}
