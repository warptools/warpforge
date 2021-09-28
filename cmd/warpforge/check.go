package main

import (
	"fmt"
	"io/ioutil"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/urfave/cli/v2"
	"github.com/warpfork/warpforge/wfapi"
)

func cmdCheck(c *cli.Context) error {
	if !c.Args().Present() {
		return fmt.Errorf("no input files provided")
	}

	for _, file := range c.Args().Slice() {
		f, err := ioutil.ReadFile(file)
		if err != nil {
			return err
		}

		t, err := getFileType(file)
		if err != nil {
			return err
		}

		var n ipld.Node
		switch t {
		case "formula":
			frmAndCtx := wfapi.FormulaAndContext{}
			n, err = ipld.Unmarshal([]byte(f), json.Decode, &frmAndCtx, wfapi.TypeSystem.TypeByName("FormulaAndContext"))
			if err != nil {
				return err
			}
		case "plot":
			plot := wfapi.Plot{}
			n, err = ipld.Unmarshal([]byte(f), json.Decode, &plot, wfapi.TypeSystem.TypeByName("Plot"))
			if err != nil {
				return err
			}
		}
		if c.Bool("verbose") {
			c.App.Metadata["result"] = n
		}
	}

	return nil
}
