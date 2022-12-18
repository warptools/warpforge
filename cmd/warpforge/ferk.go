package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/serum-errors/go-serum"
	"github.com/urfave/cli/v2"

	"github.com/warptools/warpforge/cmd/warpforge/internal/util"
	"github.com/warptools/warpforge/pkg/config"
	"github.com/warptools/warpforge/pkg/plotexec"
	"github.com/warptools/warpforge/wfapi"
)

var ferkCmdDef = cli.Command{
	Name:  "ferk",
	Usage: "Starts a containerized environment for interactive use",
	Action: util.ChainCmdMiddleware(cmdFerk,
		util.CmdMiddlewareLogging,
		util.CmdMiddlewareTracingConfig,
		util.CmdMiddlewareTracingSpan,
	),
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "plot",
			Aliases: []string{"p"},
			Usage:   "Specify a plot file to use.  If not set, a very minimal default plot with a simple base image will be used.",
		},
		&cli.StringFlag{
			Name:  "rootfs",
			Usage: "If set, assigns an input in the plot named \"rootfs\".  (This will have no effect unless the plot uses \"pipe::rootfs\" somewhere as an input.)",
		},
		&cli.StringFlag{
			Name:  "cmd",
			Usage: "If set, replaces the protoformula's action with an exec action with the specified command.  (Otherwise, the protoformula's existing action is used unchanged.)",
		},
		&cli.BoolFlag{
			Name:  "persist",
			Usage: "If set, adds a mount to the container at \"/persist\" which is read-write to the host at \"./wf-persist/\".",
		},
		&cli.BoolFlag{
			Name:  "no-interactive",
			Usage: "By default, ferk containers are interactive, and are connected to stdin.  Setting this flag closes stdin to the container immediately, making it behave more like other warpforge run modes.",
		},
	},
}

func cmdFerk(c *cli.Context) error {
	var err error
	ctx := c.Context

	plot := wfapi.Plot{}
	if c.String("plot") != "" {
		// plot was provided, load from file
		plot, err = util.PlotFromFile(c.String("plot"))
		if err != nil {
			return serum.Error(wfapi.ECodePlotInvalid, serum.WithMessageTemplate("plot file {{file}} not parsed"), serum.WithCause(err), serum.WithDetail("file", c.String("plot")))
		}
	} else {
		// no plot provided, generate the basic default plot from json template
		_, err := ipld.Unmarshal([]byte(util.FerkPlotTemplate), json.Decode, &plot, wfapi.TypeSystem.TypeByName("Plot"))
		if err != nil {
			return wfapi.ErrorSerialization("error parsing template plot", err)
		}

		// convert rootfs input string to PlotInput
		// this requires additional quoting to be parsed correctly by ipld
		if c.String("rootfs") != "" {
			// custom value provided, override default
			rootfsStr := fmt.Sprintf("\"%s\"", c.String("rootfs"))
			rootfs := wfapi.PlotInput{}
			_, err := ipld.Unmarshal([]byte(rootfsStr), json.Decode, &rootfs, wfapi.TypeSystem.TypeByName("PlotInput"))
			if err != nil {
				return wfapi.ErrorSerialization("error parsing rootfs input", err)
			}
			plot.Inputs.Values["rootfs"] = rootfs
		}
	}

	if _, exists := plot.Steps.Values["ferk"]; !exists {
		return wfapi.ErrorPlotInvalid(`requires a step named "ferk"`)
	}

	// set command to execute
	if c.String("cmd") != "" {
		plot.Steps.Values["ferk"].Protoformula.Action = wfapi.Action{
			Exec: &wfapi.Action_Exec{
				Command: strings.Split(c.String("cmd"), " "),
			},
		}
	}

	if c.Bool("persist") {
		// set up a persistent directory on the host
		sandboxPath := wfapi.SandboxPath("/persist")
		port := wfapi.SandboxPort{
			SandboxPath: &sandboxPath,
		}
		plot.Steps.Values["ferk"].Protoformula.Inputs.Keys = append(plot.Steps.Values["ferk"].Protoformula.Inputs.Keys, port)
		plot.Steps.Values["ferk"].Protoformula.Inputs.Values[port] = wfapi.PlotInput{
			PlotInputSimple: &wfapi.PlotInputSimple{
				Mount: &wfapi.Mount{
					Mode:     "rw",
					HostPath: "./wf-persist",
				},
			},
		}
		// create the persist directory, if it does not exist
		err := os.MkdirAll("wf-persist", 0755)
		if err != nil {
			return wfapi.ErrorIo("failed to create persist directory", "wf-persist", err)
		}
	}

	// set up interactive based on flags
	// disable memoization to force the formula to run
	pltCfg := wfapi.PlotExecConfig{
		FormulaExecConfig: wfapi.FormulaExecConfig{
			DisableMemoization: true,
			Interactive:        !c.Bool("no-interactive"),
		},
	}

	state, err := config.NewState()
	if err != nil {
		return err
	}
	wss, err := config.DefaultWorkspaceStack(state)
	if err != nil {
		return err
	}
	exCfg := config.PlotExecConfig(state)
	if _, err := plotexec.Exec(ctx, exCfg, wss, wfapi.PlotCapsule{Plot: &plot}, pltCfg); err != nil {
		return err
	}

	return nil
}
