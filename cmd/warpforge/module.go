package main

import (
	"fmt"
	"io/ioutil"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/urfave/cli/v2"
	"github.com/warpfork/warpforge/wfapi"
)

var moduleCmdDef = cli.Command{
	Name:  "module",
	Usage: "Subcommands that operate on modules.",
	Subcommands: []*cli.Command{
		{
			Name:   "check",
			Usage:  "Check module file(s) for syntax and sanity.",
			Action: cmdCheckModule,
		},
	},
}

func checkModule(fileName string) (*ipld.Node, error) {
	f, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	module := wfapi.Module{}
	n, err := ipld.Unmarshal([]byte(f), json.Decode, &module, wfapi.TypeSystem.TypeByName("Module"))
	return &n, err
}

func cmdCheckModule(c *cli.Context) error {
	if !c.Args().Present() {
		return fmt.Errorf("no input files provided")
	}

	for _, fileName := range c.Args().Slice() {
		n, err := checkModule(fileName)
		if err != nil {
			return err
		}
		if c.Bool("verbose") {
			c.App.Metadata["result"] = *n
		}
	}
	return nil
}
