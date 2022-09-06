package workspace

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/warpfork/warpforge/pkg/testutil"
	"github.com/warpfork/warpforge/wfapi"
)

type Workspace struct {
	fsys            fs.FS  // the fs.  (Most of the application is expected to use just one of these, but it's always configurable, largely for tests.)
	rootPath        string // workspace root path -- *not* including the magicWorkspaceDirname segment on the end.
	isHomeWorkspace bool   // if it's the ultimate workspace (the one in your homedir).
}

// OpenWorkspace returns a pointer to a Workspace object.
// It does a basic check that the workspace exists on the filesystem, but little other work;
// most info loading will be done on-demand later.
//
// OpenWorkspace assumes it will find a workspace exactly where you say; it doesn't search.
// Consider using FindWorkspace or FindWorkspaceStack in most application code.
//
// An fsys handle is required, but is typically `os.DirFS("/")` outside of tests.
//
// Errors:
//
//    - warpforge-error-workspace -- when the workspace directory fails to open
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

// OpenHomeWorkspace calls OpenWorkspace on the user's homedir.
// It will error if there's no workspace files yet there (it does not create them).
//
// An fsys handle is required, but is typically `os.DirFS("/")` outside of tests.
//
// Errors:
//
//    - warpforge-error-workspace -- when the workspace directory fails to open
func OpenHomeWorkspace(fsys fs.FS) (*Workspace, wfapi.Error) {
	return OpenWorkspace(fsys, homedir)
}

// OpenRootWorkspace calls OpenWorkspace on the first root workspace in the stack.
//
// A root workspace is marked by containing a file named "root"
//
// If no root filesystems are marked, this will default to the last item in the
// stack, which is the home workspace.
//
// An fsys handle is required, but is typically `os.DirFS("/")` outside of tests.
//
// Errors:
//
//    - warpforge-error-searching-filesystem -- when an error occurs while searching for the workspace
func OpenRootWorkspace(fsys fs.FS, basisPath string, searchPath string) (*Workspace, wfapi.Error) {
	stack, err := FindWorkspaceStack(fsys, basisPath, searchPath)
	if err != nil {
		return nil, err
	}

	for _, ws := range stack {
		if ws.IsRootWorkspace() {
			// this is our root workspace so we're done
			return ws, nil
		}
	}

	// no matches, default to the last item in the stack
	return stack[len(stack)-1], nil
}

// Returns the path for a cached ware within a workspace
// Errors:
//
//    - warpforge-error-wareid-invalid -- when a malformed WareID is provided
func (ws *Workspace) CachePath(wareId wfapi.WareID) (string, wfapi.Error) {
	if len(wareId.Hash) < 7 {
		return "", wfapi.ErrorWareIdInvalid(wareId)
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

func (ws *Workspace) IsRootWorkspace() bool {
	// check if the root marker file exists
	_, err := ws.fsys.Open(filepath.Join(ws.rootPath, magicWorkspaceDirname, "root"))
	return err == nil
}

// Returns the base path which contains memos (i.e., `.../.warpforge/memos`)
func (ws *Workspace) MemoBasePath() string {
	return filepath.Join(
		"/",
		ws.rootPath,
		".warpforge",
		"memos",
	)
}

// Returns the memo path for with a given formula ID within a workspace
func (ws *Workspace) MemoPath(fid string) string {
	return filepath.Join(
		ws.MemoBasePath(),
		strings.Join([]string{fid, "json"}, "."),
	)
}

// Returns the base path which contains named catalogs (i.e., `.../.warpforge/catalogs`)
func (ws *Workspace) CatalogBasePath() string {
	return filepath.Join(
		ws.rootPath,
		".warpforge",
		"catalogs",
	)
}

// Returns the catalog path for catalog with a given name within a workspace
func (ws *Workspace) CatalogPath(name *string) string {
	if name == nil {
		return filepath.Join(
			ws.rootPath,
			".warpforge",
			"catalog",
		)
	} else {
		return filepath.Join(
			ws.CatalogBasePath(),
			*name,
		)
	}
}

// Open a catalog within this workspace with a given name
//
// Errors:
//
//    - warpforge-error-catalog-invalid -- when opened catalog has invalid data
//    - warpforge-error-io -- when IO error occurs during opening of catalog
func (ws *Workspace) OpenCatalog(name *string) (Catalog, wfapi.Error) {
	path := ws.CatalogPath(name)
	return OpenCatalog(ws.fsys, path)
}

// List the catalogs available within a workspace
//
// Errors:
//
//    - warpforge-error-io -- when listing directory fails
func (ws *Workspace) ListCatalogs() ([]*string, wfapi.Error) {
	catalogsPath := ws.CatalogBasePath()

	_, err := fs.Stat(ws.fsys, catalogsPath)
	if os.IsNotExist(err) {
		// no catalogs directory, return an empty list
		return []*string{}, nil
	} else if err != nil {
		return []*string{}, wfapi.ErrorIo("failed to stat catalogs path", &catalogsPath, err)
	}

	// list the directory
	catalogs, err := fs.ReadDir(ws.fsys, catalogsPath)
	if err != nil {
		return []*string{}, wfapi.ErrorIo("failed to read catalogs dir", &catalogsPath, err)
	}

	// build a list of subdirectories, each is a catalog
	var list []*string
	for _, c := range catalogs {
		if c.IsDir() {
			name := c.Name()
			list = append(list, &name)
		}
	}
	return list, nil
}

// Get a catalog ware from a workspace, doing lookup by CatalogRef.
// This will first check all catalogs within the "catalogs" subdirectory, if it exists
// then, it will check the "catalog" subdirectory, if it exists
//
// Errors:
//
//     - warpforge-error-io -- when reading of lineage or mirror files fails
//     - warpforge-error-catalog-parse -- when ipld parsing of lineage or mirror files fails
//     - warpforge-error-catalog-invalid -- when ipld parsing of lineage or mirror files fails
func (ws *Workspace) GetCatalogWare(ref wfapi.CatalogRef) (*wfapi.WareID, *wfapi.WarehouseAddr, wfapi.Error) {
	// list the catalogs within the "catalogs" subdirectory
	cats, err := ws.ListCatalogs()
	if err != nil {
		return nil, nil, err
	}

	// if it exists, add the "catalog" subdirectory to the end of the list
	// this is done by adding a catalog with nil name, which refers to the unnamed catalog
	// in the "catalog" subdirectory
	catalogPath := filepath.Join(ws.rootPath, magicWorkspaceDirname, "catalog")
	_, errRaw := fs.Stat(ws.fsys, catalogPath)
	if errRaw == nil {
		// "catalog" subdirectory exists, append nil
		cats = append(cats, nil)
	}

	for _, c := range cats {
		cat, err := ws.OpenCatalog(c)
		if err != nil {
			return nil, nil, err
		}
		wareId, wareAddr, err := cat.GetWare(ref)
		if err != nil {
			return nil, nil, err
		}
		if wareId == nil {
			// not found in this catalog, keep trying
			continue
		}
		return wareId, wareAddr, nil
	}

	// nothing found
	return nil, nil, nil
}

// Check if this workspace has a catalog with a given name.
//
// Errors:
//
//     - warpforge-error-io -- when reading or writing the catalog directory fails
func (ws *Workspace) HasCatalog(name string) (bool, wfapi.Error) {
	path := ws.CatalogPath(&name)
	if _, errRaw := fs.Stat(ws.fsys, path); os.IsNotExist(errRaw) {
		return false, nil
	} else if errRaw == nil {
		return true, nil
	} else {
		return false, wfapi.ErrorIo("could not stat catalog path", &path, errRaw)
	}
}

// Create a new catalog.
// This only creates the catalog and does not open it.
//
// Errors:
//
//     - warpforge-error-io -- when reading or writing the catalog directory fails
//     - warpforge-error-catalog-invalid -- when the catalog already exists
func (ws *Workspace) CreateCatalog(name string) wfapi.Error {
	path := filepath.Join("/", ws.CatalogPath(&name))

	// check if the catalog path exists
	exists, err := ws.HasCatalog(name)
	if err != nil {
		return err
	}
	if exists {
		return wfapi.ErrorCatalogInvalid(path, "catalog already exists")
	}

	// catalog does not exist, create it
	errRaw := os.MkdirAll(path, 0755)
	if errRaw != nil {
		return wfapi.ErrorIo("could not create catalog directory", &path, errRaw)
	}

	return nil
}

// Get a catalog replay from a workspace, doing lookup by CatalogRef.
// This will first check all catalogs within the "catalogs" subdirectory, if it exists
// then, it will check the "catalog" subdirectory, if it exists
//
// Errors:
//
//     - warpforge-error-io -- when reading of lineage or mirror files fails
//     - warpforge-error-catalog-parse -- when ipld parsing of lineage or mirror files fails
//     - warpforge-error-catalog-invalid -- when ipld parsing of lineage or mirror files fails
func (ws *Workspace) GetCatalogReplay(ref wfapi.CatalogRef) (*wfapi.Plot, wfapi.Error) {
	// list the catalogs within the "catalogs" subdirectory
	cats, err := ws.ListCatalogs()
	if err != nil {
		return nil, err
	}

	// if it exists, add the "catalog" subdirectory to the end of the list
	// this is done by adding a catalog with nil name, which refers to the unnamed catalog
	// in the "catalog" subdirectory
	catalogPath := filepath.Join(ws.rootPath, magicWorkspaceDirname, "catalog")
	_, errRaw := fs.Stat(ws.fsys, catalogPath)
	if errRaw == nil {
		// "catalog" subdirectory exists, append nil
		cats = append(cats, nil)
	}

	for _, c := range cats {
		cat, err := ws.OpenCatalog(c)
		if err != nil {
			return nil, err
		}
		replay, err := cat.GetReplay(ref)
		if err != nil {
			return nil, err
		}
		if replay == nil {
			// not found in this catalog, keep trying
			continue
		}
		// found, return the replay
		return replay, nil
	}

	// nothing found
	return nil, nil
}
