package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/serum-errors/go-serum"

	"github.com/warptools/warpforge/pkg/dab"
	_ "github.com/warptools/warpforge/pkg/testutil"
	"github.com/warptools/warpforge/wfapi"
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
func OpenWorkspace(fsys fs.FS, rootPath string) (*Workspace, error) {
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
	return &Workspace{
		fsys:            fsys,
		rootPath:        rootPath,
		isRootWorkspace: checkIsRootWorkspace(fsys, rootPath),
		// that's it; everything else is loaded later.
	}
}

// openHomeWorkspace is the same as the public method but with no error checking at all;
func openHomeWorkspace(fsys fs.FS) *Workspace {
	workspace := openWorkspace(fsys, homedir)
	workspace.isHomeWorkspace = true
	workspace.isRootWorkspace = true
	return workspace
}

// OpenHomeWorkspace calls OpenWorkspace on the user's homedir.
// It will error if there's no workspace files yet there (it does not create them).
//
// An fsys handle is required, but is typically `os.DirFS("/")` outside of tests.
//
// Errors:
//
//    - warpforge-error-workspace -- when the workspace directory fails to open
func OpenHomeWorkspace(fsys fs.FS) (*Workspace, error) {
	workspace, err := OpenWorkspace(fsys, homedir)
	if err == nil {
		workspace.isHomeWorkspace = true
		workspace.isRootWorkspace = true
	}
	return workspace, err
}

// InternalPath returns the workspace's path including the .warp* segment.
func (ws *Workspace) InternalPath() string {
	if ws.isHomeWorkspace {
		return filepath.Join(ws.rootPath, magicHomeWorkspaceDirname)
	}
	return filepath.Join(ws.rootPath, magicWorkspaceDirname)
}

// Path returns the workspace's fs and path -- the directory that is its root.
// (This does *not* include the ".warp*" segment on the end of the path.)
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
func (ws *Workspace) CachePath(wareId wfapi.WareID) (string, error) {
	if len(wareId.Hash) < 7 {
		return "", wfapi.ErrorWareIdInvalid(wareId)
	}
	return filepath.Join(
		"/",
		ws.InternalPath(),
		"cache",
		string(wareId.Packtype),
		"fileset",
		wareId.Subpath(),
	), nil
}

// Returns the path to a ware within the workspace's warehouse directory
// Errors:
//
//    - warpforge-error-wareid-invalid -- when a malformed WareID is provided
func (ws *Workspace) WarePath(wareId wfapi.WareID) (string, error) {
	if len(wareId.Hash) < 7 {
		return "", wfapi.ErrorWareIdInvalid(wareId)
	}
	return filepath.Join(
		"/",
		ws.WarehousePath(),
		wareId.Subpath(),
	), nil
}

// IsRootWorkspace returns true if the workspace is a root workspace
func (ws *Workspace) IsRootWorkspace() bool {
	return ws.isRootWorkspace
}

// Returns the base path which contains memos (e.g., `.../.warpforge/memos`)
func (ws *Workspace) MemoBasePath() string {
	return filepath.Join(
		"/",
		ws.InternalPath(),
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

// Returns the base path which contains named catalogs (e.g., `.../.warpforge/catalogs`)
func (ws *Workspace) CatalogBasePath() string {
	return filepath.Join(
		ws.InternalPath(),
		"catalogs",
	)
}

// nonRootCatalogPath returns the path to the catalog in a non-root workspace.
func (ws *Workspace) nonRootCatalogPath() string {
	return filepath.Join(ws.InternalPath(), "catalog")
}

// WarehousePath returns the path to the catalog in a non-root workspace.
func (ws *Workspace) WarehousePath() string {
	return filepath.Join(ws.InternalPath(), "warehouse")
}

// CatalogPath returns the catalog path for catalog with a given name within a workspace.
// A non-root workspace must use an empty string as the catalog name.
// A root workspace must use a catalog name that matches the regular expression in the
// CatalogNameFormat package variable. A root workspace may not use an empty string catalog name.
//
// Errors:
//
//    - warpforge-error-catalog-name -- when the catalog name is invalid
func (ws *Workspace) CatalogPath(name string) (string, error) {
	if !ws.isRootWorkspace {
		if name == "" {
			return ws.nonRootCatalogPath(), nil
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
func (ws *Workspace) OpenCatalog(name string) (Catalog, error) {
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
func (ws *Workspace) ListCatalogs() ([]string, error) {
	if !ws.isRootWorkspace {
		return []string{""}, nil
	}
	catalogsPath := ws.CatalogBasePath()
	if filepath.IsAbs(catalogsPath) {
		catalogsPath = catalogsPath[1:]
	}

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
//     - warpforge-error-catalog-missing-entry -- when catalog item is missing
func (ws *Workspace) GetCatalogWare(ref wfapi.CatalogRef) (*wfapi.WareID, *wfapi.WarehouseAddr, error) {
	// list the catalogs within the "catalogs" subdirectory
	cats, err := ws.ListCatalogs()
	if err != nil {
		return nil, nil, err
	}

	for _, c := range cats {
		cat, err := ws.OpenCatalog(c)
		if err != nil {
			switch serum.Code(err) {
			case "warpforge-error-catalog-name":
				panic(err)
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
func (ws *Workspace) HasCatalog(name string) (bool, error) {
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

// CreateCatalog creates a new catalog.
// CreateCatalog only creates the catalog and does not open it.
//
// Errors:
//
//    - warpforge-error-io -- when reading or writing the catalog directory fails
//    - warpforge-error-already-exists -- when the catalog already exists
//    - warpforge-error-catalog-name -- when the catalog name is invalid
func (ws *Workspace) CreateCatalog(name string) error {
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
		return wfapi.ErrorFileAlreadyExists(path)
	}

	errRaw := os.MkdirAll(path, 0755)
	if errRaw != nil {
		return wfapi.ErrorIo("could not create catalog directory", path, errRaw)
	}

	return nil
}

// CreateOrOpenCatalog will create a catalog if it does not exist before opening
//
// Errors:
//
//  - warpforge-error-io -- when reading or writing the catalog directory fails
//  - warpforge-error-catalog-name -- when the catalog name is invalid
//  - warpforge-error-catalog-invalid -- when opened catalog has invalid data
func (ws *Workspace) CreateOrOpenCatalog(name string) (Catalog, error) {
	err := ws.CreateCatalog(name)
	if err != nil {
		switch serum.Code(err) {
		case "warpforge-error-already-exists":
			return ws.OpenCatalog(name)
		default:
			// Error Codes -= warpforge-error-already-exists
			return Catalog{}, err
		}
	}
	return ws.OpenCatalog(name)
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
func (ws *Workspace) GetCatalogReplay(ref wfapi.CatalogRef) (*wfapi.Plot, error) {
	// list the catalogs within the "catalogs" subdirectory
	cats, err := ws.ListCatalogs()
	if err != nil {
		return nil, err
	}

	for _, c := range cats {
		cat, err := ws.OpenCatalog(c)
		if err != nil {
			switch serum.Code(err) {
			case "warpforge-error-catalog-name":
				// This shouldn't happen
				panic(err)
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

// GetWarehouseAddress will return a URL-style path to the workspace warehouse.
// will use the rootpath prefixed with "/"
func (ws *Workspace) GetWarehouseAddress() wfapi.WarehouseAddr {
	path := filepath.Join("/", ws.InternalPath(), "warehouse")
	return wfapi.WarehouseAddr("ca+file://" + path)
}

// GetMirroringConfig will return the MirroringConfig map for this workspace
// which is read from the .warpforge/mirroring.json config file.
//
// Errors:
//
// 	- warpforge-error-io -- for errors reading from fsys.
// 	- warpforge-error-serialization -- for errors from try to parse the data as a Module.
func (ws *Workspace) GetMirroringConfig() (wfapi.MirroringConfig, error) {
	return dab.MirroringConfigFromFile(ws.fsys, filepath.Join(ws.InternalPath(), dab.MagicFilename_MirroringConfig))
}

// StoreMemo will save a run record to the workspace
//
// Errors:
//
//   - warpforge-error-io -- when unable to read memo file
//   - warpforge-error-serialization -- when unable to parse memo file
func (ws *Workspace) StoreMemo(rr wfapi.RunRecord) error {
	// create the memo path, if it does not exist
	memoBasePath := ws.MemoBasePath()
	err := os.MkdirAll(ws.MemoBasePath(), 0755)
	if err != nil {
		return wfapi.ErrorIo("failed to create memo dir", memoBasePath, err)
	}

	// serialize the memo
	memoSerial, err := ipld.Marshal(json.Encode, &rr, wfapi.TypeSystem.TypeByName("RunRecord"))
	if err != nil {
		return wfapi.ErrorSerialization("failed to serialize memo", err)
	}

	// write the memo
	memoPath := ws.MemoPath(rr.FormulaID)
	err = os.WriteFile(memoPath, memoSerial, 0644)
	if err != nil {
		return wfapi.ErrorIo("failed to write memo file", memoPath, err)
	}

	return nil
}

// LoadMemo will attempt to find and return a run record from the workspace
//
// Errors:
//
//   - warpforge-error-io -- when unable to read memo file
//   - warpforge-error-serialization -- when unable to parse memo file
func (ws *Workspace) LoadMemo(fid string) (*wfapi.RunRecord, error) {
	// if no workspace is provided, there can be no memos
	if ws == nil {
		return nil, nil
	}

	memoPath := ws.MemoPath(fid)
	if len(memoPath) > 0 && memoPath[0] == '/' {
		memoPath = memoPath[1:]
	}

	_, err := fs.Stat(ws.fsys, memoPath)
	if errors.Is(err, fs.ErrNotExist) {
		// couldn't find a memo file, return nil to indicate there is no memo
		return nil, nil
	}
	if err != nil {
		// found memo file, but error reading, return error
		return nil, wfapi.ErrorIo("failed to stat memo file", memoPath, err)
	}

	// read the file
	f, err := fs.ReadFile(ws.fsys, memoPath)
	if err != nil {
		return nil, wfapi.ErrorIo("failed to read memo file", memoPath, err)
	}

	memo := wfapi.RunRecord{}
	_, err = ipld.Unmarshal(f, json.Decode, &memo, wfapi.TypeSystem.TypeByName("RunRecord"))
	if err != nil {
		return nil, wfapi.ErrorSerialization(fmt.Sprintf("failed to deserialize memo file %q", memoPath), err)
	}

	return &memo, nil
}
