package healthcheckcli

import (
	"github.com/urfave/cli/v2"

	appbase "github.com/warptools/warpforge/app/base"
	"github.com/warptools/warpforge/app/base/util"
	"github.com/warptools/warpforge/pkg/healthcheck"
	"github.com/warptools/warpforge/pkg/logging"
)

func init() {
	appbase.App.Commands = append(appbase.App.Commands, healthcheckCmdDef)
}

var healthcheckCmdDef = &cli.Command{
	Name:  "healthcheck",
	Usage: "Check for potential errors in system configuration",
	Action: util.ChainCmdMiddleware(cmdHealth,
		util.CmdMiddlewareLogging,
		util.CmdMiddlewareTracingConfig,
		util.CmdMiddlewareTracingSpan,
	),
}

func cmdHealth(c *cli.Context) error {
	ctx := c.Context
	log := logging.Ctx(ctx)
	// Check tracing config
	// Check for workspace stack
	// Attempt to execute a module in a temporary workspace
	hc := &healthcheck.HealthCheck{
		Runners: []healthcheck.Runner{
			&healthcheck.KernelInfo{},
			&healthcheck.BinCheck{Name: "runc"},
			&healthcheck.BinCheck{Name: "rio"},
			&healthcheck.ExecutionInfo{},
		},
	}
	if err := hc.Run(c.Context); err != nil {
		log.Info("", "health check critical error: %s", err)
		return err
	}

	log.Debug("", "runners=%d, results=%d", len(hc.Runners), len(hc.Results))

	if err := hc.Fprint(c.App.Writer); err != nil {
		return err
	}
	return nil
}
