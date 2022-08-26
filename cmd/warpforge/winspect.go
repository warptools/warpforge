package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/warpfork/warpforge/pkg/dab"

	"github.com/urfave/cli/v2"
)

var cmdDefWorkspaceInspect = cli.Command{
	Name:   "inspect",
	Usage:  "Inspect and report upon the situation of the current workspace (how many modules are there, have we got a cached evaluation of them, etc).",
	Action: cmdFnWorkspaceInspect,
	// Aliases: []string{"winspect"}, // doesn't put them at the top level.  Womp.
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
	fs.WalkDir(wsFs, wsPath, func(path string, d fs.DirEntry, err error) error {
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

			// Tell me about it.
			fmt.Fprintf(c.App.Writer, "Module found: %q -- at path %q\n", modName, modPathWithinWs)
		}

		return nil
	})

	return nil
}
