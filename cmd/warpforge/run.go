package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/urfave/cli/v2"

	"github.com/warptools/warpforge/cmd/warpforge/internal/util"
	"github.com/warptools/warpforge/pkg/config"
	"github.com/warptools/warpforge/pkg/dab"
	"github.com/warptools/warpforge/pkg/formulaexec"
	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/wfapi"
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
	pltCfg := wfapi.PlotExecConfig{
		Recursive: c.Bool("recursive"),
		FormulaExecConfig: wfapi.FormulaExecConfig{
			DisableMemoization: c.Bool("force"),
		},
	}

	state, err := config.NewState()
	if err != nil {
		return err
	}
	logger.Debug("", "state: %#v", state)

	if !c.Args().Present() {
		filename := filepath.Join(state.WorkingDirectory, dab.MagicFilename_Module) // execute the module in the current directory
		logger.Debug("", "working directory module: %s", filename)
		_, err = util.ExecModule(ctx, state, nil, pltCfg, filename)
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
				if filepath.Base(path) == dab.MagicFilename_Module {
					if c.Bool("verbose") {
						logger.Debug("", "executing %q", path)
					}
					absPath, err := filepath.Abs(path)
					if err != nil {
						return err
					}
					state.WorkingDirectory = absPath
					_, err = util.ExecModule(ctx, state, nil, pltCfg, path)
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
		fileName, err := filepath.Abs(fileName)
		if err != nil {
			return err
		}
		if info.IsDir() {
			_, err := util.ExecModule(ctx, state, nil, pltCfg, filepath.Join(fileName, dab.MagicFilename_Module))
			if err != nil {
				return err
			}
		} else {
			// formula or module file provided
			t, err := dab.GetFileType(fileName)
			if err != nil {
				return err
			}

			switch t {
			case dab.FileType_Formula:
				// unmarshal FormulaAndContext from file data
				f, err := ioutil.ReadFile(fileName)
				if err != nil {
					return err
				}
				frmAndCtx := wfapi.FormulaAndContext{}
				_, err = ipld.Unmarshal([]byte(f), json.Decode, &frmAndCtx, wfapi.TypeSystem.TypeByName("FormulaAndContext"))
				if err != nil {
					return err
				}

				// run formula
				frmCfg := wfapi.FormulaExecConfig{}
				wss, err := config.DefaultWorkspaceStack(state)
				if err != nil {
					return err
				}
				frmExecCfg := config.FormulaExecConfig(state)
				if _, err := formulaexec.Exec(ctx, frmExecCfg, wss.Root(), frmAndCtx, frmCfg); err != nil {
					return err
				}
			case dab.FileType_Module:
				logger.Debug("", "executing module")
				_, err := util.ExecModule(ctx, state, nil, pltCfg, fileName)
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
