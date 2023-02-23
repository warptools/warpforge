package main

import (
	"context"
	"errors"
	"os"

	"github.com/serum-errors/go-serum"
	"github.com/urfave/cli/v2"

	"github.com/warptools/warpforge/cmd/warpforge/internal/util"
	"github.com/warptools/warpforge/subcmd/watch"
	"github.com/warptools/warpforge/wfapi"
)

var watchCmdDef = cli.Command{
	Name:      "watch",
	Usage:     "Watch a module for changes to plot ingest inputs. Currently only git ingests are supported.",
	UsageText: "Watch will emit execution output but will also allow communication over a unix socket via the spark command.",
	Action: util.ChainCmdMiddleware(cmdWatch,
		util.CmdMiddlewareLogging,
		util.CmdMiddlewareTracingConfig,
		util.CmdMiddlewareCancelOnInterrupt,
	),
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "disable-socket",
			Usage: "Disable unix socket server. Use this if you are having problems due to socket creation.",
		},
	},
}

func cmdWatch(c *cli.Context) error {
	if c.Args().Len() != 1 {
		return serum.Error(wfapi.ECodeInvalid, serum.WithMessageLiteral("invalid args"))
	}
	wd, err := os.Getwd()
	if err != nil {
		return serum.Error(wfapi.ECodeIo, serum.WithCause(err),
			serum.WithMessageLiteral("unable to get working directory"),
		)
	}
	cfg := &watch.Config{
		WorkingDirectory: wd,
		Fsys:             os.DirFS("/"),
		Path:             c.Args().First(),
		Socket:           !c.Bool("disable-socket"),
	}
	err = cfg.Run(c.Context)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
