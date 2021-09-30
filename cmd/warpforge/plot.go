package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/goccy/go-graphviz"
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
			Name:   "graph",
			Usage:  "Generate a graph from a plot file",
			Action: cmdPlotGraph,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "png",
					Usage: "Output graph PNG to `FILE`",
				},
			},
		},
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

func cmdPlotGraph(c *cli.Context) error {
	if c.Args().Len() != 1 {
		return fmt.Errorf("a single input file must be provided")
	}

	f, err := ioutil.ReadFile(c.Args().First())
	if err != nil {
		return err
	}

	plot := wfapi.Plot{}
	_, err = ipld.Unmarshal([]byte(f), json.Decode, &plot, wfapi.TypeSystem.TypeByName("Plot"))
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if fname := c.String("png"); fname != "" {
		err := plotexec.Graph(plot, graphviz.PNG, &buf)
		if err != nil {
			return err
		}
		os.WriteFile(fname, buf.Bytes(), 0644)
	} else {
		err := plotexec.Graph(plot, graphviz.XDOT, &buf)
		if err != nil {
			return err
		}
		fmt.Fprintln(c.App.Writer, buf.String())
	}

	return nil
}
