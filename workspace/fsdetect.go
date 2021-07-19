package workspace

import (
	"os"
	"path/filepath"

	"github.com/warpfork/warpforge/wfapi"
)

const (
	magicWorkspaceDirname = ".warpforge"
)

var homedir string

func init() {
	if homedir != "" {
		return
	}
	var err error
	homedir, err = os.UserHomeDir()
	homedir = filepath.Clean(homedir)
	if err != nil {
		wfapi.TerminalError(wfapi.ErrorSearchingFilesystem("homedir", err), 9)
	}
}

// FindWorkspace looks for a workspace on the filesystem and returns the first one found,
// searching directories upward.
//
// It searches from `join(basisPath,searchPath)` up to `basisPath`
// (in other words, it won't search above basisPath).
// Invoking it with an empty string for `basisPath` and cwd for `searchPath` is typical.
//
// If no workspace is found, it will return nil for both the workspace pointer and error value.
// If errors are returned, they're due to filesystem IO.
// FindWorkspace will refuse to return your home workspace and abort the search if it encounters it,
// returning nils in the same way as if no workspace was found.
func FindWorkspace(basisPath, searchPath string) (ws *Workspace, remainingSearchPath string, err *wfapi.Error) {
	// Our search loops over searchPath, popping a path segment off at the end of every round.
	//  Keep the given searchPath in hand; we might need it for an error report.
	searchAt := searchPath
	for {
		// Assume the search path exists and is a dir (we'll get a reasonable error anyway if it's not);
		//  join that path with our search target and try to open it.
		f, err := os.Open(filepath.Join(basisPath, searchAt, magicWorkspaceDirname))
		f.Close()
		if err == nil { // no error?  Found it!
			ws := openWorkspace(filepath.Join(basisPath, searchAt))
			if ws.isHomeWorkspace {
				ws = nil
			}
			return ws, filepath.Dir(searchAt), nil
		}
		if os.IsNotExist(err) { // no such thing?  oh well.  pop a segment and keep looking.
			searchAt = filepath.Dir(searchAt)
			// If popping a searchAt segment got us down to nothing,
			//  and we didn't find anything here either,
			//   that's it: return NotFound.
			if searchAt == "/" || searchAt == "." {
				return nil, "", nil
			}
			// ... otherwise: continue, with popped searchAt.
			continue
		}
		// You're still here?  That means there's an error, but of some unpleasant kind.
		//  Whatever this error is, our search has blind spots: error out.
		return nil, searchAt, wfapi.ErrorSearchingFilesystem("workspace", err)
	}
}

// FindWorkspaceStack works similarly to FindWorkspace, but finds all workspaces, not just the nearest one.
// The first element of the returned slice is the nearest workspace; subsequent elements are its parents, then grandparents, etc.
// The last element of the returned slice is the home workspace (or at the most extreme: where the home workspace *should be*).
func FindWorkspaceStack(basisPath, searchPath string) (wss []*Workspace, err *wfapi.Error) {
	for {
		var ws *Workspace
		ws, searchPath, err = FindWorkspace(basisPath, searchPath)
		if err != nil {
			return
		}
		if ws == nil {
			break
		}
		wss = append(wss, ws)
	}
	wss = append(wss, openWorkspace(homedir))
	return wss, nil
}

// OpenHomeWorkspace calls OpenWorkspace on the user's homedir.
// It will error if there's no workspace files yet there (it does not create them).
func OpenHomeWorkspace() (*Workspace, *wfapi.Error) {
	return OpenWorkspace(homedir)
}
