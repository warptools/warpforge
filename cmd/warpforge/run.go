package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/urfave/cli/v2"
	"github.com/warpfork/warpforge/cmd/warpforge/internal/util"
	"github.com/warpfork/warpforge/pkg/formulaexec"
	"github.com/warpfork/warpforge/pkg/logging"
	"github.com/warpfork/warpforge/wfapi"
)

var runCmdDef = cli.Command{
	Name:  "run",
	Usage: "Run a module or formula",
	Action: util.ChainCmdMiddleware(cmdRun,
		util.CmdMiddlewareLogging,
		util.CmdMiddlewareTracingConfig,
		util.CmdMiddlewareTracingSpan,
	),
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "recursive",
			Aliases: []string{"r"},
			Usage:   "Recursively execute replays required to assemble inputs to this module",
		},
		&cli.BoolFlag{
			Name:    "force",
			Aliases: []string{"f"},
			Usage:   "Force execution, even if memoized formulas exist",
		},
	},
}

func cmdRun(c *cli.Context) error {
	ctx := c.Context
	logger := logging.Ctx(ctx)
	config := wfapi.PlotExecConfig{
		Recursive: c.Bool("recursive"),
		FormulaExecConfig: wfapi.FormulaExecConfig{
			DisableMemoization: c.Bool("force"),
		},
	}

	if !c.Args().Present() {
		// execute the module in the current directory
		pwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("could not get current directory")
		}
		_, err = util.ExecModule(ctx, config, filepath.Join(pwd, util.ModuleFilename))
		if err != nil {
			return err
		}
		return nil
	}

	if filepath.Base(c.Args().First()) == "..." {
		// recursively execute module.json files
		return filepath.Walk(filepath.Dir(c.Args().First()),
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if filepath.Base(path) == util.ModuleFilename {
					if c.Bool("verbose") {
						logger.Debug("executing %q", path)
					}
					_, err = util.ExecModule(ctx, config, path)
					if err != nil {
						return err
					}
				}
				return nil
			})
	}
	// a list of individual files or directories has been provided
	for _, fileName := range c.Args().Slice() {
		info, err := os.Stat(fileName)
		if err != nil {
			return err
		}
		if info.IsDir() {
			// directory provided, execute module if it exists
			_, err := util.ExecModule(ctx, config, filepath.Join(fileName, "module.wf"))
			if err != nil {
				return err
			}
		} else {
			// formula or module file provided
			f, err := ioutil.ReadFile(fileName)
			if err != nil {
				return err
			}

			t, err := util.GetFileType(fileName)
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

				wsSet, err := util.OpenWorkspaceSet()
				if err != nil {
					return fmt.Errorf("failed to open workspace set: %s", err)
				}

				// run formula
				config := wfapi.FormulaExecConfig{}
				_, err = formulaexec.Exec(ctx, wsSet.Root(), frmAndCtx, config)
				if err != nil {
					return err
				}
			case "module":
				_, err := util.ExecModule(ctx, config, fileName)
				if err != nil {
					return err
				}
			default:
				return fmt.Errorf("unsupported file %s", fileName)
			}
		}
	}
	return nil
}
