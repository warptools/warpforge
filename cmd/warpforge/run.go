package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/urfave/cli/v2"
	"github.com/warpfork/warpforge/pkg/formulaexec"
	"github.com/warpfork/warpforge/pkg/plotexec"
	"github.com/warpfork/warpforge/pkg/workspace"
	"github.com/warpfork/warpforge/wfapi"
)

func cmdRun(c *cli.Context) error {
	if !c.Args().Present() {
		return fmt.Errorf("no input files provided")
	}

	for _, fileName := range c.Args().Slice() {
		f, err := ioutil.ReadFile(fileName)
		if err != nil {
			return err
		}

		t, err := getFileType(fileName)
		if err != nil {
			return err
		}

		switch t {
		case "formula":
			// unmarshal FormulaAndContext from file data
			frmAndCtx := wfapi.FormulaAndContext{}
			_, err = ipld.Unmarshal([]byte(f), json.Decode, &frmAndCtx, wfapi.TypeSystem.TypeByName("FormulaAndContext"))
			if err != nil {
				return err
			}

			var err error
			ws, err := workspace.OpenHomeWorkspace(os.DirFS("/"))

			// run formula
			rr, err := formulaexec.Exec(ws, frmAndCtx)
			if err != nil {
				return err
			}
			c.App.Metadata["result"] = bindnode.Wrap(&rr, wfapi.TypeSystem.TypeByName("RunRecord"))
		case "module":
			// unmarshal Module
			module := wfapi.Module{}
			_, err = ipld.Unmarshal([]byte(f), json.Decode, &module, wfapi.TypeSystem.TypeByName("Module"))
			if err != nil {
				return err
			}

			// get Plot for Module
			plotFileName := filepath.Join(filepath.Dir(fileName), "plot.json")
			f, err := ioutil.ReadFile(plotFileName)
			if err != nil {
				return err
			}

			plot := wfapi.Plot{}
			_, err = ipld.Unmarshal([]byte(f), json.Decode, &plot, wfapi.TypeSystem.TypeByName("Plot"))
			if err != nil {
				return err
			}

			pwd, err := os.Getwd()
			if err != nil {
				return err
			}
			wss, err := workspace.FindWorkspaceStack(os.DirFS("/"), "", pwd[1:])

			result, err := plotexec.Exec(wss, plot)
			if err != nil {
				return err
			}
			c.App.Metadata["result"] = bindnode.Wrap(&result, wfapi.TypeSystem.TypeByName("PlotResults"))

		default:
			return fmt.Errorf("unsupported file %s", fileName)
		}
	}
	return nil
}
