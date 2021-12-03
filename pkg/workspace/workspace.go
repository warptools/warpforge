package workspace

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/warpfork/warpforge/wfapi"
)

type Workspace struct {
	fsys            fs.FS  // the fs.  (Most of the application is expected to use just one of these, but it's always configurable, largely for tests.)
	rootPath        string // workspace root path -- *not* including the magicWorkspaceDirname segment on the end.
	isHomeWorkspace bool   // if it's the ultimate workspace (the one in your homedir).
}

// a workspace set consists of the 3 types of workspace we operate on
//  home: a workspace containing configuration information and other global info
//  root: a workspace containing catalogs to use, which also stores wares and cache
//        the home workspace is the default root workspace
//  stack: a set of workspaces that may contain additional catalogs and other project-specific info
type WorkspaceSet struct {
	Home  *Workspace
	Root  *Workspace
	Stack []*Workspace
}

// OpenWorkspace returns a pointer to a Workspace object.
// It does a basic check that the workspace exists on the filesystem, but little other work;
// most info loading will be done on-demand later.
//
// OpenWorkspace assumes it will find a workspace exactly where you say; it doesn't search.
// Consider using FindWorkspace or FindWorkspaceStack in most application code.
//
// An fsys handle is required, but is typically `os.DirFS("/")` outside of tests.
func OpenWorkspace(fsys fs.FS, rootPath string) (*Workspace, wfapi.Error) {
	f, err := fsys.Open(filepath.Join(rootPath, magicWorkspaceDirname))
	if f != nil {
		f.Close()
	}
	if err != nil {
		return nil, wfapi.ErrorWorkspace(rootPath, err)
	}
	return openWorkspace(fsys, rootPath), nil
}

// openWorkspace is the same as the public method, but with no error checking at all;
// it presumes you've already done that (as most of the Find methods have).
func openWorkspace(fsys fs.FS, rootPath string) *Workspace {
	rootPath = filepath.Clean(rootPath)
	return &Workspace{
		fsys:            fsys,
		rootPath:        rootPath,
		isHomeWorkspace: rootPath == homedir,
		// that's it; everything else is loaded later.
	}
}

// Path returns the workspace's fs and path -- the directory that is its root.
// (This does *not* include the ".warpforge" segment on the end of the path.)
func (ws *Workspace) Path() (fs.FS, string) {
	return ws.fsys, ws.rootPath
}

// IsHomeWorkspace returns true if this workspace is the one in the user's home dir.
// The home workspace is sometimes treated specially, because it's always the last one --
// it can have no parents, and is the final word for any config overrides.
// Some functions will refuse to work on the home workspace, or work specially on it.
func (ws *Workspace) IsHomeWorkspace() bool {
	return ws.isHomeWorkspace
}

// opens a full WorkspaceSet
// searches from searchPath up to basisPath for workspaces
// root workspace will be the first workspace found that is marked as a root, or the home workspace if none exists
func OpenWorkspaceSet(fsys fs.FS, basisPath string, searchPath string) (WorkspaceSet, error) {
	set := WorkspaceSet{}
	home, err := OpenHomeWorkspace(fsys)
	if err != nil {
		return set, err
	}

	root, err := OpenRootWorkspace(fsys, basisPath, searchPath)
	if err != nil {
		return set, err
	}

	stack, err := FindWorkspaceStack(fsys, basisPath, searchPath)
	if err != nil {
		return set, err
	}

	set.Home = home
	set.Root = root
	set.Stack = stack

	return set, nil
}

// returns the path for a cached ware within a workspace
func (ws *Workspace) CachePath(wareId wfapi.WareID) (string, error) {
	if len(wareId.Hash) < 7 {
		return "", fmt.Errorf("invalid WareID hash")
	}
	return filepath.Join(
		"/",
		ws.rootPath,
		".warpforge",
		"cache",
		string(wareId.Packtype),
		"fileset",
		wareId.Hash[0:3],
		wareId.Hash[3:6],
		wareId.Hash), nil
}

// returns the catalog path for catalog with a given name within a workspace
func (ws *Workspace) CatalogPath(name *string) string {
	if name == nil {
		return filepath.Join(
			"/",
			ws.rootPath,
			".warpforge",
			"catalog",
		)
	} else {
		return filepath.Join(
			"/",
			ws.rootPath,
			".warpforge",
			"catalogs",
			*name,
		)
	}
}

// list the catalogs available within a workspace
func (ws *Workspace) ListCatalogPaths() ([]string, error) {
	catalogsPath := filepath.Join(
		ws.rootPath,
		".warpforge",
		"catalogs",
	)

	_, err := fs.Stat(ws.fsys, catalogsPath)
	if os.IsNotExist(err) {
		// no catalogs directory, return an empty list
		return []string{}, nil
	} else if err != nil {
		return []string{}, err
	}

	// list the directory
	catalogs, err := fs.ReadDir(ws.fsys, catalogsPath)
	if err != nil {
		return []string{}, err
	}

	// build a list of subdirectories, each is a catalog
	var list []string
	for _, c := range catalogs {
		if c.IsDir() {
			list = append(list, filepath.Join(catalogsPath, c.Name()))
		}
	}
	return list, nil
}
