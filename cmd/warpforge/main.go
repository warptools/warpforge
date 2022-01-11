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

const VERSION = "alpha"

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
			Name: "json",
		},
	}
	app.ExitErrHandler = exitErrHandler
	app.After = afterFunc
	app.Commands = []*cli.Command{
		&formulaCmdDef,
		&moduleCmdDef,
		&plotCmdDef,
		&runCmdDef,
		&checkCmdDef,
		&catalogCmdDef,
		&watchCmdDef,
		&statusCmdDef,
		&quickstartCmdDef,
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
	err := makeApp(os.Stdin, os.Stdout, os.Stderr).Run(os.Args)
	if err != nil {
		os.Exit(1)
	}
}
