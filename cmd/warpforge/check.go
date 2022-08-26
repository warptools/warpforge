package main

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/urfave/cli/v2"
	"github.com/warpfork/warpforge/pkg/plotexec"
	"github.com/warpfork/warpforge/wfapi"
)

var checkCmdDef = cli.Command{
	Name:  "check",
	Usage: "Check file(s) for syntax and sanity",
	Action: chainCmdMiddleware(cmdCheck,
		cmdMiddlewareLogging,
		cmdMiddlewareTracingConfig,
		cmdMiddlewareTracingSpan,
	),
}

func checkModule(fileName string) (*ipld.Node, error) {
	f, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	moduleCapsule := wfapi.ModuleCapsule{}
	n, err := ipld.Unmarshal([]byte(f), json.Decode, &moduleCapsule, wfapi.TypeSystem.TypeByName("ModuleCapsule"))
	return &n, err
}

func checkPlot(ctx context.Context, fileName string) (*ipld.Node, error) {
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
	_, err = plotexec.OrderSteps(ctx, plot)

	return &n, err
}

func checkFormula(fileName string) (*ipld.Node, error) {
	f, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	frmAndCtx := wfapi.FormulaAndContext{}
	n, err := ipld.Unmarshal([]byte(f), json.Decode, &frmAndCtx, wfapi.TypeSystem.TypeByName("FormulaAndContext"))
	return &n, err
}

func cmdCheck(c *cli.Context) error {
	if !c.Args().Present() {
		return fmt.Errorf("no input files provided")
	}
	ctx := c.Context

	for _, filename := range c.Args().Slice() {
		t, err := getFileType(filename)
		if err != nil {
			return err
		}

		var n *ipld.Node
		switch t {
		case "formula":
			n, err = checkFormula(filename)
			if err != nil {
				return fmt.Errorf("%s: %s", filename, err)
			}
		case "plot":
			n, err = checkPlot(ctx, filename)
			if err != nil {
				return fmt.Errorf("%s: %s", filename, err)
			}
		case "module":
			n, err = checkModule(filename)
			if err != nil {
				return fmt.Errorf("%s: %s", filename, err)
			}
		default:
			if c.Bool("verbose") {
				fmt.Fprintf(c.App.ErrWriter, "ignoring unrecoginzed file: %q\n", filename)
			}
			continue
		}
		if c.Bool("verbose") && n != nil {
			c.App.Metadata["result"] = *n
		}
	}

	return nil
}
