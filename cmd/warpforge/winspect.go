package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"github.com/warpfork/warpforge/pkg/dab"
	"github.com/warpfork/warpforge/pkg/plotexec"
)

var cmdDefWorkspaceInspect = cli.Command{
	Name:   "inspect",
	Usage:  "Inspect and report upon the situation of the current workspace (how many modules are there, have we got a cached evaluation of them, etc).",
	Action: cmdFnWorkspaceInspect,
	// Aliases: []string{"winspect"}, // doesn't put them at the top level.  Womp.
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "gohard",
			Usage: "whether to spend effort checking the health of modules found; if false, just list them.",
			Value: true,
		},
	},
}

func cmdFnWorkspaceInspect(c *cli.Context) error {
	fsys := os.DirFS("/")

	// First, find the workspace.
	wss, err := openWorkspaceSet(fsys)
	if err != nil {
		return fmt.Errorf("failed to open workspace set: %s", err)
	}

	// Briefly report on the nearest workspace.
	// (We could talk about the grandparents too, but 'wf status' already does that; here we want to focus more on contents than parentage.)
	wsFs, wsPath := wss.Stack[0].Path()
	fmt.Fprintf(c.App.Writer, "Workspace: %s%s\n", wsFs, wsPath)

	// Search for modules within the workspace.
	return fs.WalkDir(wsFs, wsPath, func(path string, d fs.DirEntry, err error) error {
		// fmt.Fprintf(c.App.Writer, "hi: %s%s\n", wsFs, path)

		if err != nil {
			return err
		}

		// Don't ever look into warpforge guts directories.
		if d.Name() == dab.MagicFilename_Workspace {
			return fs.SkipDir
		}

		// If this is a dir (beyond the root): look see if it contains a workspace marker.
		// If it does, we might not want to report on it.
		// TODO: a bool flag for this.
		if d.IsDir() && len(path) > len(wsPath) {
			_, e2 := fs.Stat(wsFs, filepath.Join(path, dab.MagicFilename_Workspace))
			if e2 == nil || os.IsNotExist(e2) {
				// carry on
			} else {
				return fs.SkipDir
			}
		}

		// Peek for module file.
		if d.Name() == dab.MagicFilename_Module {
			modPathWithinWs := path[len(wsPath)+1 : len(path)-len(dab.MagicFilename_Module)] // leave the trailing slash on.  For disambig in case we support multiple module files per dir someday.
			mod, err := dab.ModuleFromFile(wsFs, path)
			modName := mod.Name
			if err != nil {
				modName = "!!Unknown!!"
			}

			everythingParses := false
			importsResolve := false
			noticeIngestUsage := false
			noticeMountUsage := false
			havePacksCached := false // maybe should have a variant for "or we have a replay we're hopeful about"?
			haveRunrecord := false
			haveHappyExit := false
			if c.Bool("gohard") {
				if err != nil {
					goto _checksDone
				}
				plot, err := dab.PlotFromFile(wsFs, filepath.Join(filepath.Dir(path), dab.MagicFilename_Plot))
				if err != nil {
					goto _checksDone
				}
				everythingParses = true
				plotStats, err := plotexec.ComputeStats(plot, wss)
				if err != nil {
					return err // if it's hardcore catalog errors, rather than just unresolvables, I'm out
				}
				if plotStats.ResolvableCatalogInputs == plotStats.InputsUsingCatalog {
					importsResolve = true
				}
				if plotStats.InputsUsingIngest > 0 {
					noticeIngestUsage = true
				}
				if plotStats.InputsUsingMount > 0 {
					noticeMountUsage = true
				}
				// TODO: havePacksCached is not supported right now :(
				// TODO: haveRunrecord needs to both do resolve, and go peek at memos, and yet (obviously) not actually run.
				// TODO: haveHappyExit needs the above.
			}
		_checksDone:

			// Tell me about it.
			fmt.Fprintf(c.App.Writer, "Module found: %q -- at path %q", modName, modPathWithinWs)
			if c.Bool("gohard") {
				fmt.Fprintf(c.App.Writer, " -- %v %v %v %v %v %v %v",
					everythingParses, importsResolve, noticeIngestUsage, noticeMountUsage, havePacksCached, haveRunrecord, haveHappyExit)
			}
			fmt.Fprintf(c.App.Writer, "\n")
		}

		return nil
	})
}
