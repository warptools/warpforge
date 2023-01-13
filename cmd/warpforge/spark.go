package main

import (
	"context"
	"errors"
	"fmt"
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
			Name:  "format-markup",
			Value: string(spark.DefaultMarkup),
			Usage: "Set output format." + fmt.Sprintf("%s", spark.MarkupList),
		},
		&cli.StringFlag{
			Name:  "format-style",
			Value: string(spark.DefaultStyle),
			Usage: "Set output format." + fmt.Sprintf("%s", spark.StyleList),
		},
		&cli.BoolFlag{Name: "no-color"},
	},
}

// Errors:
//
//  - warpforge-error-io --
func getwd() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", serum.Error(wfapi.ECodeIo, serum.WithCause(err))
	}
	return wd, nil
}

// Errors:
//
//    - warpforge-error-invalid -- argument error
//    - warpforge-error-io -- working directory error
//    - warpforge-spark-no-module -- module not found
//    - warpforge-spark-no-socket -- when socket does not dial or does not exist
//    - warpforge-spark-unknown -- all other errors
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
		OutputMarkup:     c.String("format-markup"),
		OutputStyle:      c.String("format-style"),
		OutputColor:      !c.Bool("no-color"),
	}
	err = cfg.Run(c.Context)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
