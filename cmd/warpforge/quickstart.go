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
		return fmt.Errorf("no module name provided")
	}

	_, err := os.Stat("module.json")
	if !os.IsNotExist(err) {
		return fmt.Errorf("module.json file already exists")
	}
	_, err = os.Stat("plot.json")
	if !os.IsNotExist(err) {
		return fmt.Errorf("plot.json file already exists")
	}

	moduleName := c.Args().First()

	module := wfapi.Module{
		Name: wfapi.ModuleName(moduleName),
	}
	moduleSerial, err := ipld.Marshal(json.Encode, &module, wfapi.TypeSystem.TypeByName("Module"))
	if err != nil {
		return fmt.Errorf("failed to serialize module")
	}
	os.WriteFile("module.json", moduleSerial, 0644)

	plot := wfapi.Plot{}
	_, err = ipld.Unmarshal([]byte(defaultPlotJson), json.Decode, &plot, wfapi.TypeSystem.TypeByName("Plot"))
	if err != nil {
		return fmt.Errorf("failed to deserialize default plot")
	}
	plotSerial, err := ipld.Marshal(json.Encode, &plot, wfapi.TypeSystem.TypeByName("Plot"))
	if err != nil {
		return fmt.Errorf("failed to serialize plot")
	}
	os.WriteFile("plot.json", plotSerial, 0644)

	return nil
}
