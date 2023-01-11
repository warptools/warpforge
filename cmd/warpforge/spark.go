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
	Flags: []cli.Flag{},
}

func wdOrArg(c *cli.Context, argIdx int) (string, error) {
	if wd := c.Args().Get(argIdx); wd != "" {
		return wd, nil
	}
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
func cmdSpark(c *cli.Context) error {
	if c.Args().Len() > 1 {
		return serum.Errorf(wfapi.ECodeInvalid, "too many args")
	}
	wd, err := wdOrArg(c, 0)
	if err != nil {
		return err
	}
	cfg := &spark.Config{
		WorkingDirectory: wd,
	}
	err = cfg.Run(c.Context)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
