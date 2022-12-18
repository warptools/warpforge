package config

import (
	"errors"
	"io/fs"
	"path/filepath"

	"github.com/warptools/warpforge/pkg/workspace"
	"github.com/warptools/warpforge/wfapi"
)

func findLocalModule(fsys fs.FS, workingDir string, wss workspace.WorkspaceSet) (string, error) {
	localWorkspace := wss[0]
	_, path := localWorkspace.Path()
	modulePath, _, err := FindModule(fsys, path, workingDir)
	if err != nil {
		return "", err
	}
	return modulePath, nil
}

// DefaultModuleFilename is the string used to detect module files
const DefaultModuleFilename = "module.wf"

// FindModule looks for a module file on the filesystem and returns the first one found,
// searching directories upward.
//
// It searches from `join(basisPath,searchPath)` up to `basisPath`
// (in other words, it won't search above basisPath).
// Invoking it with an empty string for `basisPath` and cwd for `searchPath` is typical.
//
// If no module file is found, it will return nil for the error value.
// If errors are returned, they're due to filesystem IO.
//
// An fsys handle is required, but is typically `os.DirFS("/")` outside of tests.
//
// Errors:
//
//    - warpforge-error-searching-filesystem -- when an unexpected error occurs traversing the search path
func FindModule(fsys fs.FS, basisPath, searchPath string) (path string, remainingSearchPath string, err error) {
	// Our search loops over searchPath, popping a path segment off at the end of every round.
	//  Keep the given searchPath in hand; we might need it for an error report.
	searchAt := searchPath
	for {
		path := filepath.Join(basisPath, searchAt, DefaultModuleFilename)
		_, err := fs.Stat(fsys, path)
		if err == nil {
			return path, filepath.Dir(searchAt), nil
		}
		if errors.Is(err, fs.ErrNotExist) { // no such thing?  oh well.  pop a segment and keep looking.
			searchAt = filepath.Dir(searchAt)
			// If popping a searchAt segment got us down to nothing,
			//  and we didn't find anything here either,
			//   that's it: return NotFound.
			if searchAt == "/" || searchAt == "." {
				return "", "", nil
			}
			// ... otherwise: continue, with popped searchAt.
			continue
		}
		// You're still here?  That means there's an error, but of some unpleasant kind.
		//  Whatever this error is, our search has blind spots: error out.
		return "", searchAt, wfapi.ErrorSearchingFilesystem("workspace", err)
	}
}
