package main

import (
	"fmt"
	"os"
	"strings"

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
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name: "rootfs",
		},
		&cli.StringFlag{
			Name: "cmd",
		},
	},
}

type ferkPlot struct {
	Rootfs  string
	Shell   string
	Persist bool
}

const defaultRootfs = "catalog:warpsys.org/bootstrap-rootfs:bullseye-1646092800:amd64"

const ferkPlotTemplate = `
{
        "inputs": {
                "rootfs": "literal:none"
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

	// generate the basic default plot from json template
	plot := wfapi.Plot{}
	_, err = ipld.Unmarshal([]byte(ferkPlotTemplate), json.Decode, &plot, wfapi.TypeSystem.TypeByName("Plot"))
	if err != nil {
		return fmt.Errorf("error parsing template plot: %s", err)
	}

	// convert rootfs input string to PlotInput
	// this requires additional quoting to be parsed correctly by ipld
	rootfsStr := fmt.Sprintf("\"%s\"", defaultRootfs)
	if c.String("rootfs") != "" {
		// custom value provided, override default
		rootfsStr = fmt.Sprintf("\"%s\"", c.String("rootfs"))
	}
	rootfs := wfapi.PlotInput{}
	_, err = ipld.Unmarshal([]byte(rootfsStr), json.Decode, &rootfs, wfapi.TypeSystem.TypeByName("PlotInput"))
	if err != nil {
		return fmt.Errorf("error parsing rootfs input: %s", err)
	}
	plot.Inputs.Values["rootfs"] = rootfs

	// set command to execute
	if c.String("cmd") != "" {
		plot.Steps.Values["ferk"].Protoformula.Action.Exec.Command = strings.Split(c.String("cmd"), " ")
	}

	// create the persist directory, if it does not exist
	os.MkdirAll("wf-persist", 0755)

	// run the plot in interactive mode
	config := wfapi.PlotExecConfig{
		Interactive: true,
	}
	_, err = plotexec.Exec(wss, plot, config, logger)
	if err != nil {
		return err
	}

	return nil
}
