package appbase

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/urfave/cli/v2"

	_ "github.com/warptools/warpforge/app/base/helpgen"
)

const VERSION = "v0.4.0"

var App = &cli.App{
	Name:    "warpforge",
	Version: VERSION,
	Usage:   "the everything-builder and any-environment manager",

	Reader:    closedReader{}, // Replace with os.Stdin in real application; or other wiring, in tests.
	Writer:    panicWriter{},  // Replace with os.Stdout in real application; or other wiring, in tests.
	ErrWriter: panicWriter{},  // Replace with os.Stderr in real application; or other wiring, in tests.

	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			EnvVars: []string{"WARPFORGE_DEBUG"},
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
	},

	// The commands slice is updated by each package that contains commands.
	// Import the parent of this package to get that all done for you!
	Commands: []*cli.Command{},

	ExitErrHandler: func(c *cli.Context, err error) {
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
	},
}

// Aaaand the other modifications to `urfave/cli` that are unfortunately only possible by manipulating globals:
func init() {
	cli.VersionFlag = &cli.BoolFlag{
		Name:               "version", // And no short aliases.  "-v" is for "verbose"!
		Usage:              "print the version",
		DisableDefaultText: true,
	}
}

type closedReader struct{}

// Read is a dummy method that always returns EOF.
func (c closedReader) Read(p []byte) (int, error) {
	return 0, io.EOF
}

type panicWriter struct{}

// Write is a dummy method that always panics.  You're supposed to replace panicWriter values before use.
func (p panicWriter) Write(data []byte) (int, error) {
	panic("replace the Writer and ErrWriter on the App value in packages that use it!")
}
