package main

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/urfave/cli/v2"

	"github.com/warptools/warpforge/cmd/warpforge/internal/util"
	"github.com/warptools/warpforge/pkg/plotexec"
	"github.com/warptools/warpforge/wfapi"
)

var checkCmdDef = cli.Command{
	Name:  "check",
	Usage: "Check file(s) for syntax and sanity",
	Action: util.ChainCmdMiddleware(cmdCheck,
		util.CmdMiddlewareLogging,
		util.CmdMiddlewareTracingConfig,
		util.CmdMiddlewareTracingSpan,
	),
}

func checkModule(fileName string) (*ipld.Node, error) {
	f, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, wfapi.ErrorIo("cannot read module file", fileName, err)
	}

	moduleCapsule := wfapi.ModuleCapsule{}
	n, err := ipld.Unmarshal([]byte(f), json.Decode, &moduleCapsule, wfapi.TypeSystem.TypeByName("ModuleCapsule"))
	if err != nil {
		return nil, wfapi.ErrorSerialization("cannot deserialize module", err)
	}
	return &n, nil
}

func checkPlot(ctx context.Context, fileName string) (*ipld.Node, error) {
	f, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, wfapi.ErrorIo("cannot read plot file", fileName, err)
	}

	// parse the Plot
	plot := wfapi.Plot{}
	n, err := ipld.Unmarshal([]byte(f), json.Decode, &plot, wfapi.TypeSystem.TypeByName("Plot"))
	if err != nil {
		return nil, wfapi.ErrorSerialization("cannot deserialize plot", err)
	}

	// ensure Plot order can be resolved
	if _, err := plotexec.OrderSteps(ctx, plot); err != nil {
		return &n, err
	}

	return &n, nil
}

func checkFormula(fileName string) (*ipld.Node, error) {
	f, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, wfapi.ErrorIo("cannot read formula file", fileName, err)
	}

	frmAndCtx := wfapi.FormulaAndContext{}
	n, err := ipld.Unmarshal([]byte(f), json.Decode, &frmAndCtx, wfapi.TypeSystem.TypeByName("FormulaAndContext"))
	if err != nil {
		return nil, wfapi.ErrorSerialization("cannot deserialize formula", err)
	}
	return &n, nil
}

func cmdCheck(c *cli.Context) error {
	if !c.Args().Present() {
		return fmt.Errorf("no input files provided")
	}
	ctx := c.Context

	for _, filename := range c.Args().Slice() {
		t, err := util.GetFileType(filename)
		if err != nil {
			return err
		}

		var n *ipld.Node
		switch t {
		case "formula":
			n, err = checkFormula(filename)
			if err != nil {
				return err
			}

		case "plot":
			n, err = checkPlot(ctx, filename)
			if err != nil {
				return err
			}
		case "module":
			n, err = checkModule(filename)
			if err != nil {
				return err
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
