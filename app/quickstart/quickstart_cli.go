package quickstartcli

import (
	"fmt"
	"os"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/urfave/cli/v2"

	appbase "github.com/warptools/warpforge/app/base"
	"github.com/warptools/warpforge/app/base/util"
	"github.com/warptools/warpforge/pkg/dab"
	"github.com/warptools/warpforge/wfapi"
)

func init() {
	appbase.App.Commands = append(appbase.App.Commands, quickstartCmdDef)
}

var quickstartCmdDef = &cli.Command{
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
	if err = os.WriteFile(dab.MagicFilename_Module, moduleSerial, 0644); err != nil {
		return wfapi.ErrorIo("failed to write module file", dab.MagicFilename_Module, err)
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
	if err := os.WriteFile(dab.MagicFilename_Plot, plotSerial, 0644); err != nil {
		return wfapi.ErrorIo("failed to write plot file", dab.MagicFilename_Plot, err)
	}
	return nil
}

func cmdQuickstart(c *cli.Context) error {
	if c.Args().Len() != 1 {
		fmt.Fprintf(c.App.ErrWriter, "no module name provided\n\nA module name is an identifier. Typically one looks like 'foo.org/group/theproject', but any name will do.")
		return fmt.Errorf("no module name provided")
	}

	_, err := os.Stat(dab.MagicFilename_Module)
	if !os.IsNotExist(err) {
		return fmt.Errorf("%s file already exists", dab.MagicFilename_Module)
	}
	_, err = os.Stat(dab.MagicFilename_Plot)
	if !os.IsNotExist(err) {
		return fmt.Errorf("%s file already exists", dab.MagicFilename_Plot)
	}

	moduleName := c.Args().First()

	if err := createQuickstartFiles(moduleName); err != nil {
		return err
	}

	if !c.Bool("quiet") {
		fmt.Fprintf(c.App.Writer, "Successfully created %s and %s for module %q.\n", dab.MagicFilename_Module, dab.MagicFilename_Plot, moduleName)
		fmt.Fprintf(c.App.Writer, "Ensure your catalogs are up to date by running `%s catalog update`.\n", c.App.Name)
		fmt.Fprintf(c.App.Writer, "You can check status of this module with `%s status`.\n", c.App.Name)
		fmt.Fprintf(c.App.Writer, "You can run this module with `%s run`.\n", c.App.Name)
		fmt.Fprintf(c.App.Writer, "Once you've run the Hello World example, edit the 'script' section of %s to customize what happens.\n", dab.MagicFilename_Plot)
	}

	return nil
}
