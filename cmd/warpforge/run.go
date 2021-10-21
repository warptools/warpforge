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

var runCmdDef = cli.Command{
	Name:   "run",
	Usage:  "Run a module or formula",
	Action: cmdRun,
}

func execModule(c *cli.Context, fileName string, data []byte) (wfapi.PlotResults, error) {
	result := wfapi.PlotResults{}
	// unmarshal Module
	module := wfapi.Module{}
	_, err := ipld.Unmarshal(data, json.Decode, &module, wfapi.TypeSystem.TypeByName("Module"))
	if err != nil {
		return result, err
	}

	// get Plot for Module
	plotFileName := filepath.Join(filepath.Dir(fileName), "plot.json")
	f, err := ioutil.ReadFile(plotFileName)
	if err != nil {
		return result, err
	}

	plot := wfapi.Plot{}
	_, err = ipld.Unmarshal(f, json.Decode, &plot, wfapi.TypeSystem.TypeByName("Plot"))
	if err != nil {
		return result, err
	}

	pwd, err := os.Getwd()
	if err != nil {
		return result, err
	}

	// override the workspace search path if env var is set
	var searchPath string
	path, override := os.LookupEnv("WARPFORGE_WORKSPACE")
	if override {
		searchPath = path
	} else {
		searchPath = pwd
	}
	wss, err := workspace.FindWorkspaceStack(os.DirFS("/"), "", searchPath[1:])
	if err != nil {
		return result, err
	}

	err = os.Chdir(filepath.Dir(fileName))
	if err != nil {
		return result, err
	}
	result, err = plotexec.Exec(wss, plot)
	cdErr := os.Chdir(pwd)
	if cdErr != nil {
		return result, cdErr
	}
	if err != nil {
		return result, err
	}

	return result, nil
}

func cmdRun(c *cli.Context) error {
	if !c.Args().Present() {
		return fmt.Errorf("no input files provided")
	}

	if filepath.Base(c.Args().First()) == "..." {
		// recursively execute module.json files
		return filepath.Walk(filepath.Dir(c.Args().First()),
			func(path string, info os.FileInfo, err error) error {
				if filepath.Base(path) == "module.json" {
					if c.Bool("verbose") {
						fmt.Printf("executing %q\n", path)
					}
					f, err := ioutil.ReadFile(path)
					if err != nil {
						return err
					}
					_, err = execModule(c, path, f)
					if err != nil {
						return err
					}
				}
				return nil
			})
	} else {
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
				result, err := execModule(c, fileName, f)
				if err != nil {
					return err
				}
				c.App.Metadata["result"] = bindnode.Wrap(&result, wfapi.TypeSystem.TypeByName("PlotResults"))
			default:
				return fmt.Errorf("unsupported file %s", fileName)
			}
		}
	}
	return nil
}
