package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/serum-errors/go-serum"
	"github.com/urfave/cli/v2"

	"github.com/warptools/warpforge/cmd/warpforge/internal/util"
	"github.com/warptools/warpforge/pkg/config"
	"github.com/warptools/warpforge/pkg/dab"
	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/pkg/plotexec"
	"github.com/warptools/warpforge/pkg/workspace"
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
			Usage:   "Specify a plot file or module directory to use.  The current directory is used by default.  If the current directory isn't a module, or if this flag is explicitly set to empty string, a very minimal default plot with a simple base image will be used.",
			Value:   ".",
		},
		&cli.StringFlag{
			Name:    "step",
			Aliases: []string{"s"},
			Usage:   "Name a step in the plot that we want to run interactively.  Any other steps leading up to it will be evaluated noninteractively.",
			Value:   "ferk",
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
	log := logging.Ctx(ctx)

	plot, plotDir, err := cmdFerk_selectPlot(c)
	if err != nil {
		return err
	}

	stepName := c.String("step")
	// FUTURE: this isn't supporting nested subplots, and probably should.
	if _, exists := plot.Steps.Values[wfapi.StepName(stepName)]; !exists {
		return serum.Error(wfapi.ECodeArgument,
			serum.WithMessageTemplate("the step requested for interactivity -- {{step|q}} -- is not present in the plot"),
			serum.WithDetail("step", stepName),
		)
	}

	// apply rootfs flag if applicable
	if c.String("rootfs") != "" {
		// custom value provided, override default
		rootfsStr := fmt.Sprintf("\"%s\"", c.String("rootfs"))
		rootfs := wfapi.PlotInput{}
		// TODO this is a silly way to go about this: assigning the string to a representation-level builder is much more direct and correct.
		_, err = ipld.Unmarshal([]byte(rootfsStr), json.Decode, &rootfs, wfapi.TypeSystem.TypeByName("PlotInput"))
		if err != nil {
			return wfapi.ErrorSerialization("error parsing rootfs input", err)
		}
		plot.Inputs.Values["rootfs"] = rootfs
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

	exCfg, err := config.PlotExecConfig(&plotDir)
	if err != nil {
		return err
	}
	wss, err := workspace.FindWorkspaceStack(os.DirFS("/"), "", exCfg.WorkingDirectory)
	if err != nil {
		return err
	}
	log.Debug("", "working directory: %s", exCfg.WorkingDirectory)
	for idx, ws := range wss {
		log.Debug("", "ws %d: %q; home: %t; root: %t", idx, ws.InternalPath(), ws.IsHomeWorkspace(), ws.IsRootWorkspace())
		wsfs, path := ws.Path()
		log.Debug("", "ws %d: %q", idx, path)
		rp := filepath.Join(ws.InternalPath(), "root")
		_, err := os.Stat(rp)
		log.Debug("", "ws %d: %q -> %t", idx, rp, err == nil)
		_, err = fs.Stat(wsfs, rp[1:])
		log.Debug("", "ws %d: %q -> %t", idx, rp, err == nil)
	}

	_, err = plotexec.Exec(ctx, exCfg, wss, wfapi.PlotCapsule{Plot: plot}, pltCfg)
	if err != nil {
		return err
	}

	return nil
}

func cmdFerk_selectPlot(c *cli.Context) (plot *wfapi.Plot, plotDir string, err error) {
	// If emptystring plot: make a default one.
	if c.String("plot") == "" {
		_, err = ipld.Unmarshal([]byte(util.FerkPlotTemplate), json.Decode, &plot, wfapi.TypeSystem.TypeByName("Plot"))
		if err != nil {
			panic(err) // This is a fixed value at compile time.  It can't fail unless there's a bug.
		}
		return
	}

	// Look for module and plot files.
	// If we received a filename: expect that to be a plot.
	//   (In this case, we'll have no idea what a module name is!)
	// If it's a dir: look for a module, and then look for its plot.
	pth, err := filepath.Abs(c.String("plot"))
	if err != nil {
		return
	}
	m, p, f, _, _, err := dab.FindActionableFromFS(os.DirFS("/"), pth, "", false, dab.ActionableSearch_Any)
	_, _ = m, f // TODO support these
	if err != nil {
		return nil, "", err
	}
	if p == nil {
		return nil, "", serum.Error(wfapi.ECodeMissing,
			serum.WithMessageTemplate("could not find a plot given path {{path|q}}"),
			serum.WithDetail("path", pth),
		)
	}
	return p, pth, err
}
