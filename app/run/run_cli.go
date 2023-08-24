package runcli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/serum-errors/go-serum"
	"github.com/urfave/cli/v2"
	"github.com/warpfork/go-fsx"
	"github.com/warpfork/go-fsx/osfs"

	appbase "github.com/warptools/warpforge/app/base"
	"github.com/warptools/warpforge/app/base/util"
	"github.com/warptools/warpforge/pkg/config"
	"github.com/warptools/warpforge/pkg/dab"
	"github.com/warptools/warpforge/pkg/formulaexec"
	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/pkg/workspace"
	"github.com/warptools/warpforge/wfapi"
)

func init() {
	appbase.App.Commands = append(appbase.App.Commands, runCmdDef)
}

var runCmdDef = &cli.Command{
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

type runTargets struct {
	fs   fsx.FS
	list []*runTarget
}
type runTarget struct {
	originalRequest string // The user-given argument that resulted in this target.  A filesystem path fragment, generally.  Might be "." or "someformula.wf" or "./foo/...".  Can be the same for multiple run targets.
	mainFilename    string // The actual filename to consult.  May contain a module, or a plot, or a formula -- we don't know yet -- but we've at least checked that it exists.
	isModule        bool   // Set to true if we picked this file in such a way that it really has to contain a module.  (Not any kind of security boundary, but is often true, and elides some guessing at later stages.)
}

func (rts *runTargets) append(rt runTarget) {
	rts.list = append(rts.list, &rt)
}

// findRunTargets turns CLI args into a set of paths for each thing that the args described.
// This might be quite a few things: the args can include a list, but also a "..." can imply a whole directory walk.
//
// For all the requests that are specific (i.e. not using "..."), we check that the file exists -- you probably want to hear about any typos before we start launching into heavy duty work.
// For any requests that are using "...", we do the directory walk up-front.  This lets us estimate how much work is about to happen.
// We don't actually load or parse any files yet -- just check existence.
//
// Nonexistent specific requests result in errors.
// A "..." that has no matches produces no comment.
// The first error encountered causes return; we do not accumulate multiple errors.
//
// TODO: this probably should be extracted to `pkg/dab`.
func findRunTargets(args cli.Args, fs fsx.FS) (results runTargets, err error) {
	results = runTargets{
		fs: fs,
	}

	// If there were no positional args at all: we'll take that as meaning "try to do the cwd, as a module".
	// FUTURE:TODO: this should probably use `SearchFSAndLoadActionable` -- so that it "DTRT" if used in a subdir of a module.
	if !args.Present() {
		filename := filepath.Join(".", dab.MagicFilename_Module)
		results.append(runTarget{
			originalRequest: ".",
			mainFilename:    filename,
			isModule:        true,
		})
		if isFile, _ := fsx.IsPathFile(results.fs, filename); !isFile {
			err = serum.Errorf(wfapi.ECodeArgument, "cannot run nothing; no module file exists in current directory.  (Hint: Module files should have the name %q.)", dab.MagicFilename_Module)
		}
		return
	}

	// Loop over all the args.  They're cumulative.
	for _, arg := range args.Slice() {
		// If we have a "...": do a walk.  Gather any files with the name expected for modules.
		if filepath.Base(arg) == "..." {
			e2 := fsx.WalkDir(fs, filepath.Dir(arg),
				func(path string, _ fsx.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if filepath.Base(path) == dab.MagicFilename_Module {
						results.append(runTarget{
							originalRequest: arg,
							mainFilename:    path,
							isModule:        true,
						})
					}
					return nil
				},
			)
			if e2 != nil {
				err = serum.Errorf(wfapi.ECodeArgument, "error while walking for modules matching %q: %w", arg, e2)
				return
			}
			continue
		}

		// This one's a path to some single file or directory, then.
		fi, e2 := os.Stat(arg)
		if e2 != nil {
			err = serum.Errorf(wfapi.ECodeArgument, "error looking for runnable content at %q: %w", arg, e2)
			return
		}
		if fi.IsDir() { // If it's a dir, we'll look for module files.
			filename := filepath.Join(arg, dab.MagicFilename_Module)
			if isFile, _ := fsx.IsPathFile(results.fs, filename); !isFile {
				err = serum.Errorf(wfapi.ECodeArgument, "cannot run anything at %q: since it's a directory, expected a module file.  (Hint: Module files should have the name %q.)", arg, dab.MagicFilename_Module)
				return
			}
			results.append(runTarget{
				originalRequest: arg,
				mainFilename:    filename,
				isModule:        true,
			})
		} else { // We'll presume plain file, then.
			// This could contain a formula, or a module.  We don't inspect that at this stage.
			results.append(runTarget{
				originalRequest: arg,
				mainFilename:    arg,
			})
		}
	}
	return
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

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	logger.Debug("", "pwd: %s", cwd)

	rts, err := findRunTargets(c.Args(), osfs.DirFS("."))
	if err != nil {
		return err
	}
	for _, target := range rts.list {
		fullPath := filepath.Join(cwd, target.mainFilename)
		if target.isModule {
			logger.Debug("", "executing module from file %q, as requested by the argument %q", fullPath, target.originalRequest)
			_, err := util.ExecModule(ctx, nil, pltCfg, fullPath)
			if err != nil {
				return err
			}
		} else {
			t, err := dab.GetFileType(target.mainFilename) // FIXME this is based on filename; `dab.GuessDocumentType` would probably do more useful things.
			if err != nil {
				return err
			}

			switch t {
			case dab.FileType_Formula:
				logger.Debug("", "executing formula from file %q, as requested by the argument %q", fullPath, target.originalRequest)
				// unmarshal FormulaAndContext from file data
				f, err := fs.ReadFile(rts.fs, target.mainFilename)
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
				wss, err := workspace.FindWorkspaceStack(os.DirFS("/"), "", cwd)
				if err != nil {
					return err
				}
				formulaDir := filepath.Dir(filepath.Join(cwd, target.mainFilename))
				frmExecCfg, err := config.FormulaExecConfig(&formulaDir)
				if err != nil {
					return err
				}
				if _, err := formulaexec.Exec(ctx, frmExecCfg, wss.Root(), frmAndCtx, frmCfg); err != nil {
					return err
				}
			case dab.FileType_Module:
				logger.Debug("", "executing module from file %q, as requested by the argument %q", fullPath, target.originalRequest)
				_, err := util.ExecModule(ctx, nil, pltCfg, fullPath)
				if err != nil {
					return err
				}
			default:
				return fmt.Errorf("unsupported file %s", fullPath)
			}
		}
	}
	return nil
}
