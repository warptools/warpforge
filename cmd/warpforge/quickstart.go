package main

import (
	"fmt"
	"os"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/urfave/cli/v2"

	"github.com/warptools/warpforge/cmd/warpforge/internal/util"
	"github.com/warptools/warpforge/wfapi"
)

var quickstartCmdDef = cli.Command{
	Name:  "quickstart",
	Usage: "Generate a basic module and plot",
	Action: util.ChainCmdMiddleware(cmdQuickstart,
		util.CmdMiddlewareLogging,
		util.CmdMiddlewareTracingConfig,
		util.CmdMiddlewareTracingSpan,
	),
}

func createQuickstartFiles(moduleName string) error {
	moduleCapsule := wfapi.ModuleCapsule{
		Module: &wfapi.Module{
			Name: wfapi.ModuleName(moduleName),
		},
	}
	moduleSerial, err := ipld.Marshal(json.Encode, &moduleCapsule, wfapi.TypeSystem.TypeByName("ModuleCapsule"))
	if err != nil {
		return wfapi.ErrorSerialization("failed to serialize module", err)
	}
	if err = os.WriteFile(util.ModuleFilename, moduleSerial, 0644); err != nil {
		return wfapi.ErrorIo("failed to write module file", util.ModuleFilename, err)
	}
	plotCapsule := wfapi.PlotCapsule{}
	_, err = ipld.Unmarshal([]byte(util.DefaultPlotJson), json.Decode, &plotCapsule, wfapi.TypeSystem.TypeByName("PlotCapsule"))
	if err != nil {
		return wfapi.ErrorSerialization("failed to deserialize default plot", err)
	}

	plotSerial, err := ipld.Marshal(json.Encode, &plotCapsule, wfapi.TypeSystem.TypeByName("PlotCapsule"))
	if err != nil {
		return wfapi.ErrorSerialization("failed to serialize plot", err)
	}
	if err := os.WriteFile(util.PlotFilename, plotSerial, 0644); err != nil {
		return wfapi.ErrorIo("failed to write plot file", util.PlotFilename, err)
	}
	return nil
}

func cmdQuickstart(c *cli.Context) error {
	if c.Args().Len() != 1 {
		fmt.Fprintf(c.App.ErrWriter, "no module name provided\n\nA module name is an identifier. Typically one looks like 'foo.org/group/theproject', but any name will do.")
		return fmt.Errorf("no module name provided")
	}

	_, err := os.Stat(util.ModuleFilename)
	if !os.IsNotExist(err) {
		return fmt.Errorf("%s file already exists", util.ModuleFilename)
	}
	_, err = os.Stat(util.PlotFilename)
	if !os.IsNotExist(err) {
		return fmt.Errorf("%s file already exists", util.PlotFilename)
	}

	moduleName := c.Args().First()

	if err := createQuickstartFiles(moduleName); err != nil {
		return err
	}

	if !c.Bool("quiet") {
		fmt.Fprintf(c.App.Writer, "Successfully created %s and %s for module %q.\n", util.ModuleFilename, util.PlotFilename, moduleName)
		fmt.Fprintf(c.App.Writer, "Ensure your catalogs are up to date by running `%s catalog update`.\n", os.Args[0])
		fmt.Fprintf(c.App.Writer, "You can check status of this module with `%s status`.\n", os.Args[0])
		fmt.Fprintf(c.App.Writer, "You can run this module with `%s run`.\n", os.Args[0])
		fmt.Fprintf(c.App.Writer, "Once you've run the Hello World example, edit the 'script' section of %s to customize what happens.\n", util.PlotFilename)
	}

	return nil
}
