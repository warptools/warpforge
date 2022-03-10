package main

import (
	"fmt"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/urfave/cli/v2"
	"github.com/warpfork/warpforge/pkg/logging"
	"github.com/warpfork/warpforge/pkg/plotexec"
	"github.com/warpfork/warpforge/wfapi"
)

var ferkCmdDef = cli.Command{
	Name:   "ferk",
	Usage:  "Idk yet",
	Action: cmdFerk,
}

var ferkPlot = `
{
        "inputs": {
                "rootfs": "catalog:warpsys.org/bootstrap-rootfs:bullseye-1646092800:amd64"
        },
        "steps": {
                "build": {
                        "protoformula": {
                                "inputs": {
                                        "/": "pipe::rootfs",
                                        "/pwd": "mount:overlay:.",
                                },
                                "action": {
                                        "exec": {
                                                "command": ["/bin/bash"],
												"network": true
                                        }
                                },
                                "outputs": {}
                        }
                }
        },
        "outputs": {}
}
`

func cmdFerk(c *cli.Context) error {
	result := wfapi.PlotResults{}

	logger := logging.NewLogger(c.App.Writer, c.App.ErrWriter, c.Bool("verbose"))

	wss, err := openWorkspaceSet()
	if err != nil {
		return err
	}

	plot := wfapi.Plot{}
	_, err = ipld.Unmarshal([]byte(ferkPlot), json.Decode, &plot, wfapi.TypeSystem.TypeByName("Plot"))
	if err != nil {
		return err
	}

	config := wfapi.PlotExecConfig{
		Recursive: c.Bool("recursive"),
	}
	result, err = plotexec.Exec(wss, plot, config, logger)
	if err != nil {
		return err
	}

	fmt.Println(result)
	return nil
}
