package main

import (
	"fmt"
	"io/ioutil"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/urfave/cli/v2"
	"github.com/warpfork/warpforge/pkg/plotexec"
	"github.com/warpfork/warpforge/wfapi"
)

var plotCmdDef = cli.Command{
	Name:  "plot",
	Usage: "Subcommands that operate on plots.",
	Subcommands: []*cli.Command{
		{
			Name:   "check",
			Usage:  "Check plot file(s) for syntax and sanity.",
			Action: cmdCheckPlot,
		},
	},
}

func checkPlot(fileName string) (*ipld.Node, error) {
	f, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	// parse the Plot
	plot := wfapi.Plot{}
	n, err := ipld.Unmarshal([]byte(f), json.Decode, &plot, wfapi.TypeSystem.TypeByName("Plot"))
	if err != nil {
		return nil, err
	}

	// ensure Plot order can be resolved
	_, err = plotexec.OrderSteps(plot)

	return &n, err
}

func cmdCheckPlot(c *cli.Context) error {
	if !c.Args().Present() {
		return fmt.Errorf("no input files provided")
	}

	for _, fileName := range c.Args().Slice() {
		n, err := checkPlot(fileName)
		if err != nil {
			return err
		}
		if c.Bool("verbose") {
			c.App.Metadata["result"] = *n
		}
	}
	return nil
}
