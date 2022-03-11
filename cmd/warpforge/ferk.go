package main

import (
	"os"

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
                "ferk": {
                        "protoformula": {
                                "inputs": {
                                        "/": "pipe::rootfs",
                                        "/pwd": "mount:overlay:.",
                                        "/persist": "mount:rw:wf-persist",
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

	os.MkdirAll("wf-persist", 0755)

	config := wfapi.PlotExecConfig{
		Recursive:   c.Bool("recursive"),
		Interactive: true,
	}
	_, err = plotexec.Exec(wss, plot, config, logger)
	if err != nil {
		return err
	}

	return nil
}
