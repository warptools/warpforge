package workspace

import (
	"fmt"
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
	isRootWorkspace bool   // if it's a root workspace.
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
	_, err := statDir(fsys, filepath.Join(rootPath, magicWorkspaceDirname))
	if err != nil {
		return nil, wfapi.ErrorWorkspace(rootPath, err)
	}
	return openWorkspace(fsys, rootPath), nil
}

// openWorkspace is the same as the public method, but with no error checking at all;
// it presumes you've already done that (as most of the Find methods have).
//
// Changing the filesystem or home directory won't affect the status of whether this workspace
// is considered a root workspace or the home workspace respectively after opening. This should
// prevent an active workspace set from losing its root workspace at the cost of inconsistent state
// from an outside perspective.
func openWorkspace(fsys fs.FS, rootPath string) *Workspace {
	rootPath = filepath.Clean(rootPath)
	isHomeWorkspace := rootPath == homedir
	return &Workspace{
		fsys:            fsys,
		rootPath:        rootPath,
		isHomeWorkspace: isHomeWorkspace,
		isRootWorkspace: checkIsRootWorkspace(fsys, rootPath) || isHomeWorkspace,
		// that's it; everything else is loaded later.
	}
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

// IsRootWorkspace returns true if the workspace is a root workspace
func (ws *Workspace) IsRootWorkspace() bool {
	return ws.isRootWorkspace
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

// defaultCatalogPath returns the path to the default catalog belonging to this workspace.
func (ws *Workspace) defaultCatalogPath() string {
	return filepath.Join(ws.rootPath, ".warpforge", "catalog")
}

// Returns the catalog path for catalog with a given name within a workspace.
// Guards against filepath modifying names by considering any path which
// would be modified by filepath.Clean to be considered invalid.
// Guards against catalog nesting by considering any name with filepath separators to be invalid.
//
// Errors:
//
//    - warpforge-error-catalog-name -- when the catalog name is invalid
func (ws *Workspace) CatalogPath(name string) (string, wfapi.Error) {
	if !ws.isRootWorkspace {
		if name == "" {
			return ws.defaultCatalogPath(), nil
		}
		return "", wfapi.ErrorCatalogName(name, "named catalogs must be in a root workspace")
	}
	if name == "" {
		return "", wfapi.ErrorCatalogName("", "catalogs for a root workspace must have a non-empty name")
	}
	basePath := ws.CatalogBasePath()
	catalogPath := filepath.Join(basePath, name)

	if !reCatalogName.MatchString(name) {
		return "", wfapi.ErrorCatalogName(name, fmt.Sprintf("catalog name must match expression: %s", reCatalogName))
	}
	return catalogPath, nil
}

// Open a catalog within this workspace with a given name
//
// Errors:
//
//    - warpforge-error-catalog-invalid -- when opened catalog has invalid data
//    - warpforge-error-io -- when IO error occurs during opening of catalog
//    - warpforge-error-catalog-name -- when the catalog name is invalid
func (ws *Workspace) OpenCatalog(name string) (Catalog, wfapi.Error) {
	path, err := ws.CatalogPath(name)
	if err != nil {
		return Catalog{}, err
	}
	return OpenCatalog(ws.fsys, path)
}

// List the catalogs available within a workspace
// Will skip catalogs with invalid names
// A non-root workspace will only return the empty catalog name
//
// Errors:
//
//    - warpforge-error-io -- when listing directory fails
func (ws *Workspace) ListCatalogs() ([]string, wfapi.Error) {
	if !ws.isRootWorkspace {
		return []string{""}, nil
	}
	catalogsPath := ws.CatalogBasePath()

	_, err := fs.Stat(ws.fsys, catalogsPath)
	if os.IsNotExist(err) {
		// no catalogs directory, return an empty list
		return []string{}, nil
	} else if err != nil {
		return []string{}, wfapi.ErrorIo("failed to stat catalogs path", catalogsPath, err)
	}

	// list the directory
	catalogs, err := fs.ReadDir(ws.fsys, catalogsPath)
	if err != nil {
		return []string{}, wfapi.ErrorIo("failed to read catalogs dir", catalogsPath, err)
	}

	// build a list of subdirectories, each is a catalog
	var list []string
	for _, c := range catalogs {
		if c.IsDir() && reCatalogName.MatchString(c.Name()) {
			name := c.Name()
			list = append(list, name)
		}
	}
	return list, nil
}

// Get a catalog ware from a workspace, doing lookup by CatalogRef.
// In a root workspace this will check valid catalogs within the "catalogs" subdirectory
// In a non-root workspace, it will check the "catalog" subdirectory
//
// Errors:
//
//     - warpforge-error-io -- when reading of lineage or mirror files fails
//     - warpforge-error-catalog-parse -- when ipld parsing of lineage or mirror files fails
//     - warpforge-error-catalog-invalid -- when ipld parsing of lineage or mirror files fails
//     - warpforge-error-internal -- when the catalog name is invalid, this shouldn't happen
func (ws *Workspace) GetCatalogWare(ref wfapi.CatalogRef) (*wfapi.WareID, *wfapi.WarehouseAddr, wfapi.Error) {
	// list the catalogs within the "catalogs" subdirectory
	cats, err := ws.ListCatalogs()
	if err != nil {
		return nil, nil, err
	}

	for _, c := range cats {
		cat, err := ws.OpenCatalog(c)
		if err != nil {
			switch err.(*wfapi.ErrorVal).Code() {
			case "warpforge-error-catalog-name":
				err := wfapi.ErrorInternal("referenced an invalid catalog", err)
				return nil, nil, err
			default:
				// Error Codes -= warpforge-error-catalog-name
				return nil, nil, err
			}
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
//     - warpforge-error-catalog-name -- when the catalog name is invalid
func (ws *Workspace) HasCatalog(name string) (bool, wfapi.Error) {
	path, err := ws.CatalogPath(name)
	if err != nil {
		return false, err
	}
	_, errRaw := fs.Stat(ws.fsys, path)
	if os.IsNotExist(errRaw) {
		return false, nil
	}
	if errRaw != nil {
		return false, wfapi.ErrorIo("could not stat catalog path", path, errRaw)
	}
	return true, nil
}

// Create a new catalog.
// This only creates the catalog and does not open it.
//
// Errors:
//
//     - warpforge-error-io -- when reading or writing the catalog directory fails
//     - warpforge-error-catalog-invalid -- when the catalog already exists
//     - warpforge-error-catalog-name -- when the catalog name is invalid
func (ws *Workspace) CreateCatalog(name string) wfapi.Error {
	// catalog does not exist, create it
	path, err := ws.CatalogPath(name)
	if err != nil {
		return err
	}
	path = filepath.Join("/", path)

	// check if the catalog path exists
	exists, err := ws.HasCatalog(name)
	if err != nil {
		return err
	}
	if exists {
		return wfapi.ErrorCatalogInvalid(path, "catalog already exists")
	}

	errRaw := os.MkdirAll(path, 0755)
	if errRaw != nil {
		return wfapi.ErrorIo("could not create catalog directory", path, errRaw)
	}

	return nil
}

// Get a catalog replay from a workspace, doing lookup by CatalogRef.
// In a root workspace this will check valid catalogs within the "catalogs" subdirectory
// In a non-root workspace, it will check the "catalog" subdirectory
//
// Errors:
//
//     - warpforge-error-io -- when reading of lineage or mirror files fails
//     - warpforge-error-catalog-parse -- when ipld parsing of lineage or mirror files fails
//     - warpforge-error-catalog-invalid -- when ipld parsing of lineage or mirror files fails
//     - warpforge-error-internal -- when the catalog name is invalid, this shouldn't happen
func (ws *Workspace) GetCatalogReplay(ref wfapi.CatalogRef) (*wfapi.Plot, wfapi.Error) {
	// list the catalogs within the "catalogs" subdirectory
	cats, err := ws.ListCatalogs()
	if err != nil {
		return nil, err
	}

	for _, c := range cats {
		cat, err := ws.OpenCatalog(c)
		if err != nil {
			switch err.(*wfapi.ErrorVal).Code() {
			case "warpforge-error-catalog-name":
				// This shouldn't happen
				err := wfapi.ErrorInternal("referenced an invalid catalog", err)
				return nil, err
			default:
				// Error Codes -= warpforge-error-catalog-name
				return nil, err
			}
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
