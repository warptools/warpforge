package main

import (
	"context"
	"errors"
	"os"

	"github.com/serum-errors/go-serum"
	"github.com/urfave/cli/v2"

	"github.com/warptools/warpforge/cmd/warpforge/internal/util"
	"github.com/warptools/warpforge/pkg/plumbing/spark"
	"github.com/warptools/warpforge/wfapi"
)

var sparkCmdDef = cli.Command{
	Name:  "spark",
	Usage: "TODO", // TODO:
	Action: util.ChainCmdMiddleware(cmdSpark,
		util.CmdMiddlewareLogging,
		util.CmdMiddlewareTracingConfig,
		util.CmdMiddlewareCancelOnInterrupt,
	),
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "format",
			Value: "simple",
			Usage: "Set output format.",
		},
		&cli.BoolFlag{Name: "no-color"},
	},
}

func getwd() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", serum.Error(wfapi.ECodeIo, serum.WithCause(err))
	}
	return wd, nil
}

// Errors:
//
//   - warpforge-error-io --
//   - warpforge-error-invalid --
//   - warpforge-error-query --
//   - warpforge-error-serialization --
//   - warpforge-error-io-dial --
//   - warpforge-error-searching-filesystem --
//   - warpforge-error-unknown --
func cmdSpark(c *cli.Context) error {
	if c.Args().Len() > 1 {
		return serum.Errorf(wfapi.ECodeInvalid, "too many args")
	}
	wd, err := getwd()
	if err != nil {
		return err
	}
	cfg := &spark.Config{
		Fsys:             os.DirFS("/"),
		Path:             c.Args().Get(0),
		WorkingDirectory: wd,
		OutputStyle:      c.String("format"),
		OutputColor:      !c.Bool("no-color"),
	}
	err = cfg.Run(c.Context)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
