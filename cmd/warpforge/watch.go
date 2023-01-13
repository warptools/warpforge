package main

import (
	"context"
	"errors"
	"os"

	"github.com/serum-errors/go-serum"
	"github.com/urfave/cli/v2"

	"github.com/warptools/warpforge/cmd/warpforge/internal/util"
	"github.com/warptools/warpforge/pkg/plumbing/watch"
	"github.com/warptools/warpforge/wfapi"
)

var watchCmdDef = cli.Command{
	Name:  "watch",
	Usage: "Watch a directory for git commits, executing plot on each new commit",
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
		PlotConfig: wfapi.PlotExecConfig{
			Recursive: c.Bool("recursive"),
			FormulaExecConfig: wfapi.FormulaExecConfig{
				DisableMemoization: c.Bool("force"),
			},
		},
	}
	err = cfg.Run(c.Context)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
