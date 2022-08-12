package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/urfave/cli/v2"
	"github.com/warpfork/warpforge/pkg/formulaexec"
	"github.com/warpfork/warpforge/pkg/logging"
	"github.com/warpfork/warpforge/pkg/plotexec"
	"github.com/warpfork/warpforge/pkg/workspace"
	"github.com/warpfork/warpforge/wfapi"
	"go.opentelemetry.io/otel"
)

var runCmdDef = cli.Command{
	Name:   "run",
	Usage:  "Run a module or formula",
	Action: cmdRun,
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

func execModule(ctx context.Context, config wfapi.PlotExecConfig, fileName string) (wfapi.PlotResults, error) {
	result := wfapi.PlotResults{}

	// parse the module, even though it is not currently used
	_, err := moduleFromFile(fileName)
	if err != nil {
		return result, err
	}

	plot, err := plotFromFile(filepath.Join(filepath.Dir(fileName), PLOT_FILE_NAME))
	if err != nil {
		return result, err
	}

	pwd, err := os.Getwd()
	if err != nil {
		return result, err
	}

	wss, err := openWorkspaceSet()
	if err != nil {
		return result, err
	}

	err = os.Chdir(filepath.Dir(fileName))
	if err != nil {
		return result, err
	}

	result, err = plotexec.Exec(ctx, wss, wfapi.PlotCapsule{Plot: &plot}, config)
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
	logger := logging.NewLogger(c.App.Writer, c.App.ErrWriter, c.Bool("json"), c.Bool("quiet"), c.Bool("verbose"))
	ctx := logger.WithContext(c.Context)

	traceProvider, err := configTracer(c.String("trace"))
	if err != nil {
		return fmt.Errorf("could not initialize tracing: %w", err)
	}
	defer traceShutdown(c.Context, traceProvider)
	tr := otel.Tracer(TRACER_NAME)
	ctx, span := tr.Start(ctx, c.Command.FullName())
	defer span.End()

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
		_, err = execModule(ctx, config, filepath.Join(pwd, MODULE_FILE_NAME))
		if err != nil {
			return err
		}
	} else if filepath.Base(c.Args().First()) == "..." {
		// recursively execute module.json files
		return filepath.Walk(filepath.Dir(c.Args().First()),
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if filepath.Base(path) == MODULE_FILE_NAME {
					if c.Bool("verbose") {
						logger.Debug("executing %q", path)
					}
					_, err = execModule(ctx, config, path)
					if err != nil {
						return err
					}
				}
				return nil
			})
	} else {
		// a list of individual files or directories has been provided
		for _, fileName := range c.Args().Slice() {
			info, err := os.Stat(fileName)
			if err != nil {
				return err
			}
			if info.IsDir() {
				// directory provided, execute module if it exists
				_, err := execModule(ctx, config, filepath.Join(fileName, "module.wf"))
				if err != nil {
					return err
				}
			} else {
				// formula or module file provided
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
					config := wfapi.FormulaExecConfig{}
					_, err = formulaexec.Exec(ctx, ws, frmAndCtx, config)
					if err != nil {
						return err
					}
				case "module":
					_, err := execModule(ctx, config, fileName)
					if err != nil {
						return err
					}
				default:
					return fmt.Errorf("unsupported file %s", fileName)
				}
			}
		}
	}
	return nil
}
