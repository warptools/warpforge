package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/warpfork/warpforge/pkg/workspace"
)

// Returns the file type, which is the file name without extension
// e.g., formula.wf -> formula, module.wf -> module, etc...
func getFileType(name string) (string, error) {
	split := strings.Split(filepath.Base(name), ".")
	if len(split) != 2 {
		// ignore files without extensions
		return "", fmt.Errorf("unsupported file: %q", name)
	}
	return split[0], nil
}

// helper function for finding the path to internally used binaries (e.g, rio, runc)
func binPath(bin string) (string, error) {
	path, override := os.LookupEnv("WARPFORGE_PATH")
	if override {
		return filepath.Join(path, bin), nil
	}

	path, err := os.Executable()
	if err != nil {
		return "", err
	}

	return filepath.Join(filepath.Dir(path), bin), nil
}

// Opens the default WorkspaceSet.
// This consists of:
// stack: a workspace stack starting at the current working directory,
// root workspace: the first marked root workspace in the stack, or the home workspace if none are marked,
// home workspace: the workspace at the user's homedir
func openWorkspaceSet(fsys fs.FS) (workspace.WorkspaceSet, error) {
	pwd, err := os.Getwd() // FIXME why are you doing this again?  you almost certainly already did it moments ago.
	if err != nil {
		return workspace.WorkspaceSet{}, fmt.Errorf("failed to get working directory: %s", err)
	}

	wss, err := workspace.OpenWorkspaceSet(fsys, "", pwd[1:])
	if err != nil {
		return workspace.WorkspaceSet{}, fmt.Errorf("failed to open workspace: %s", err)
	}
	return wss, nil
}
