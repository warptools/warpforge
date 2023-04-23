package sparkcli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/serum-errors/go-serum"
	"github.com/urfave/cli/v2"

	appbase "github.com/warptools/warpforge/app/base"
	"github.com/warptools/warpforge/app/base/util"
	"github.com/warptools/warpforge/subcmd/spark"
	"github.com/warptools/warpforge/wfapi"
)

func init() {
	appbase.App.Commands = append(appbase.App.Commands, sparkCmdDef)
}

var sparkCmdDef = &cli.Command{
	Name:  "spark",
	Usage: "Experimental RPC for getting module build status from the watch server",
	UsageText: strings.Join([]string{
		"Will attempt to find a module within a workspace and query for the build status of that module.",
		"You may set the output markup based on the needed output. For example, use bash markup to put the output in your terminal prompt.",
		"You may set the output style for different output text: 'api' for raw codes, 'phase' for short ascii strings, 'pretty' for short unicode strings.",
	}, "\n   "),
	Description: "[module path]: The search path for the module within a workspace. Will default to the current directory if not set.",
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
		&cli.BoolFlag{
			Name:  "no-color",
			Usage: "Disables colored output. This mostly is equivalent to markup=none.",
		},
	},
	ArgsUsage: "[module path]",
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
//   - warpforge-spark-no-workspace -- can't find workspace
//   - warpforge-spark-no-module -- module not found
//   - warpforge-spark-no-socket -- when socket does not dial or does not exist
//   - warpforge-spark-internal -- all other errors
//   - warpforge-spark-server -- server response was an error
//   - warpforge-error-invalid -- invalid arguments
//   - warpforge-error-io -- unable to get working directory
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
		SearchPath:       c.Args().Get(0),
		WorkingDirectory: wd,
		OutputMarkup:     c.String("format-markup"),
		OutputStyle:      c.String("format-style"),
		OutputColor:      !c.Bool("no-color"),
		OutputStream:     c.App.Writer,
	}
	err = cfg.Run(c.Context)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
