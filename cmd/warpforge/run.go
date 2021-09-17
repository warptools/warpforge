package main

import (
	"fmt"
	"io/ioutil"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/urfave/cli/v2"
	"github.com/warpfork/warpforge/pkg/formulaexec"
	"github.com/warpfork/warpforge/wfapi"
)

func cmdRun(c *cli.Context) error {
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

		switch t {
		case "formula":
			frmAndCtx := wfapi.FormulaAndContext{}
			_, err = ipld.Unmarshal([]byte(f), json.Decode, &frmAndCtx, wfapi.TypeSystem.TypeByName("FormulaAndContext"))
			if err != nil {
				return err
			}
			rr, err := formulaexec.Exec(frmAndCtx)
			if err != nil {
				return err
			}
			c.App.Metadata["result"] = bindnode.Wrap(&rr, wfapi.TypeSystem.TypeByName("RunRecord"))
		default:
			return fmt.Errorf("unsupported file %s", file)
		}
	}
	return nil
}
