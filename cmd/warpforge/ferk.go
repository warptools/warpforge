package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/urfave/cli/v2"

	"github.com/warpfork/warpforge/cmd/warpforge/internal/util"
	"github.com/warpfork/warpforge/pkg/plotexec"
	"github.com/warpfork/warpforge/wfapi"
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
			Name: "rootfs",
		},
		&cli.StringFlag{
			Name: "cmd",
		},
		&cli.BoolFlag{
			Name: "persist",
		},
		&cli.BoolFlag{
			Name: "no-interactive",
		},
		&cli.StringFlag{
			Name:    "plot",
			Aliases: []string{"p"},
		},
	},
}

func cmdFerk(c *cli.Context) error {
	var err error
	ctx := c.Context
	wss, err := util.OpenWorkspaceSet()
	if err != nil {
		return err
	}

	plot := wfapi.Plot{}
	if c.String("plot") != "" {
		// plot was provided, load from file
		plot, err = util.PlotFromFile(c.String("plot"))
		if err != nil {
			return err
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
	config := wfapi.PlotExecConfig{
		FormulaExecConfig: wfapi.FormulaExecConfig{
			DisableMemoization: true,
			Interactive:        !c.Bool("no-interactive"),
		},
	}
	if _, err := plotexec.Exec(ctx, wss, wfapi.PlotCapsule{Plot: &plot}, config); err != nil {
		return err
	}

	return nil
}
