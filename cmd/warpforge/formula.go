package main

import (
	"fmt"
	"io/ioutil"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/urfave/cli/v2"
	"github.com/warpfork/warpforge/wfapi"
)

var formulaCmdDef = cli.Command{
	Name:  "formula",
	Usage: "Subcommands that operate on formulas.",
	Subcommands: []*cli.Command{
		{
			Name:   "check",
			Usage:  "Check formula file(s) for syntax and sanity.",
			Action: cmdCheckFormula,
		},
	},
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

func cmdCheckFormula(c *cli.Context) error {
	if !c.Args().Present() {
		return fmt.Errorf("no input files provided")
	}

	for _, fileName := range c.Args().Slice() {
		n, err := checkFormula(fileName)
		if err != nil {
			return err
		}
		if c.Bool("verbose") {
			c.App.Metadata["result"] = *n
		}
	}
	return nil
}
