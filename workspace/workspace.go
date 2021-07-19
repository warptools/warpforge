package workspace

import (
	"os"
	"path/filepath"

	"github.com/warpfork/warpforge/wfapi"
)

type Workspace struct {
	rootPath        string // workspace root path -- *not* including the magicWorkspaceDirname segment on the end.
	isHomeWorkspace bool   // if it's the ultimate workspace (the one in your homedir).
}

// OpenWorkspace returns a pointer to a Workspace object.
// It does a basic check that the workspace exists on the filesystem, but little other work;
// most info loading will be done on-demand later.
//
// OpenWorkspace assumes it will find a workspace exactly where you say; it doesn't search.
// Consider using FindWorkspace or FindWorkspaceStack in most application code.
func OpenWorkspace(rootPath string) (*Workspace, *wfapi.Error) {
	f, err := os.Open(filepath.Join(rootPath, magicWorkspaceDirname))
	f.Close()
	if err != nil {
		return nil, wfapi.ErrorWorkspace(rootPath, err)
	}
	return openWorkspace(rootPath), nil
}

// openWorkspace is the same as the public method, but with no error checking at all;
// it presumes you've already done that (as most of the Find methods have).
func openWorkspace(rootPath string) *Workspace {
	rootPath = filepath.Clean(rootPath)
	return &Workspace{
		rootPath:        rootPath,
		isHomeWorkspace: rootPath == homedir,
		// that's it; everything else is loaded later.
	}
}

// Path returns the workspace's path -- the directory that is its root.
// (This does *not* include the ".warpforge" segment on the end of the path.)
func (ws *Workspace) Path() string {
	return ws.rootPath
}

// IsHomeWorkspace returns true if this workspace is the one in the user's home dir.
// The home workspace is sometimes treated specially, because it's always the last one --
// it can have no parents, and is the final word for any config overrides.
// Some functions will refuse to work on the home workspace, or work specially on it.
func (ws *Workspace) IsHomeWorkspace() bool {
	return ws.isHomeWorkspace
}
