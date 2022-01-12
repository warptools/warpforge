package main

import (
	"fmt"
	"os"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/urfave/cli/v2"
	"github.com/warpfork/warpforge/wfapi"
)

var quickstartCmdDef = cli.Command{
	Name:   "quickstart",
	Usage:  "Generate a basic module and plot.",
	Action: cmdQuickstart,
}

const defaultPlotJson = `{
	"inputs": {
		"rootfs": "catalog:alpinelinux.org/alpine:v3.14.2:x86_64"
	},
	"steps": {
		"hello-world": {
			"protoformula": {
				"inputs": {
					"/": "pipe::rootfs"
				},
				"action": {
					"script": {
						"interpreter": "/bin/sh",
						"contents": [
							"mkdir /output",
							"echo 'hello world' | tee /output/file"
						],
						"network": false
					}
				},
				"outputs": {
					"out": {
						"from": "/output",
						"packtype": "tar"
					}
				}
			}
		}
	},
	"outputs": {
		"output": "pipe:hello-world:out"
	}
}
`

func cmdQuickstart(c *cli.Context) error {
	if c.Args().Len() != 1 {
		fmt.Fprintf(c.App.ErrWriter, "no module name provided\n\nA module name is an identifier. Typically one looks like 'foo.org/group/theproject', but any name will do.")
		return fmt.Errorf("no module name provided")
	}

	_, err := os.Stat(MODULE_FILE_NAME)
	if !os.IsNotExist(err) {
		return fmt.Errorf("%s file already exists", MODULE_FILE_NAME)
	}
	_, err = os.Stat(PLOT_FILE_NAME)
	if !os.IsNotExist(err) {
		return fmt.Errorf("%s file already exists", PLOT_FILE_NAME)
	}

	moduleName := c.Args().First()

	module := wfapi.Module{
		Name: wfapi.ModuleName(moduleName),
	}
	moduleSerial, err := ipld.Marshal(json.Encode, &module, wfapi.TypeSystem.TypeByName("Module"))
	if err != nil {
		return fmt.Errorf("failed to serialize module")
	}
	err = os.WriteFile(MODULE_FILE_NAME, moduleSerial, 0644)
	if err != nil {
		return fmt.Errorf("failed to write module.json file: %s", err)
	}

	plot := wfapi.Plot{}
	_, err = ipld.Unmarshal([]byte(defaultPlotJson), json.Decode, &plot, wfapi.TypeSystem.TypeByName("Plot"))
	if err != nil {
		return fmt.Errorf("failed to deserialize default plot")
	}
	plotSerial, err := ipld.Marshal(json.Encode, &plot, wfapi.TypeSystem.TypeByName("Plot"))
	if err != nil {
		return fmt.Errorf("failed to serialize plot")
	}

	err = os.WriteFile(PLOT_FILE_NAME, plotSerial, 0644)
	if err != nil {
		return fmt.Errorf("failed to write %s: %s", PLOT_FILE_NAME, err)
	}

	fmt.Fprintf(c.App.Writer, "Successfully created %s and %s for module %q.\n", MODULE_FILE_NAME, PLOT_FILE_NAME, moduleName)
	fmt.Fprintf(c.App.Writer, "Ensure your catalogs are up to date by running `%s catalog update.`.\n", os.Args[0])
	fmt.Fprintf(c.App.Writer, "You can check status of this module with `%s status`.\n", os.Args[0])
	fmt.Fprintf(c.App.Writer, "You can run this module with `%s run`.\n", os.Args[0])
	fmt.Fprintf(c.App.Writer, "Once you've run the Hello World example, edit the 'script' section of %s to customize what happens.\n", PLOT_FILE_NAME)

	return nil
}
