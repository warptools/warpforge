package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/warpfork/warpforge/wfapi"
)

const (
	magicWorkspaceDirname = ".warpforge"
)

var homedir string

func init() {
	var err error
	// Assign homedir.
	//  Somewhat complicated by the fact we non-rooted paths internally for consistency
	//   (which is in turn driven largely by stdlib's `testing/testfs` not supporting them).
	homedir, err = os.UserHomeDir()
	homedir = filepath.Clean(homedir)
	if err != nil {
		wfapi.TerminalError(wfapi.ErrorSearchingFilesystem("homedir", err), 9)
	}
	if homedir == "" {
		homedir = "home" // dummy, just to avoid the irritant of empty strings.
	}
	if homedir[0] == '/' { // de-rootify this, for ease of comparison with other derootified paths.
		homedir = homedir[1:]
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
//
// An fsys handle is required, but is typically `os.DirFS("/")` outside of tests.
//
// Errors:
//
//    - warpforge-error-searching-filesystem -- when an unexpected error occurs traversing the search path
func FindWorkspace(fsys fs.FS, basisPath, searchPath string) (ws *Workspace, remainingSearchPath string, err wfapi.Error) {
	// Our search loops over searchPath, popping a path segment off at the end of every round.
	//  Keep the given searchPath in hand; we might need it for an error report.
	searchAt := searchPath
	for {
		// Assume the search path exists and is a dir (we'll get a reasonable error anyway if it's not);
		//  join that path with our search target and try to open it.
		f, err := fsys.Open(filepath.Join(basisPath, searchAt, magicWorkspaceDirname))
		if f != nil {
			f.Close()
		}
		if err == nil { // no error?  Found it!
			ws := openWorkspace(fsys, filepath.Join(basisPath, searchAt))
			if ws.isHomeWorkspace {
				ws = nil
			}
			return ws, filepath.Dir(searchAt), nil
		}
		if errors.Is(err, fs.ErrNotExist) { // no such thing?  oh well.  pop a segment and keep looking.
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
//
// An fsys handle is required, but is typically `os.DirFS("/")` outside of tests.
//
// Errors:
//
//    - warpforge-error-searching-filesystem -- when an unexpected error occurs traversing the search path
func FindWorkspaceStack(fsys fs.FS, basisPath, searchPath string) (wss []*Workspace, err wfapi.Error) {
	// Repeatedly apply FindWorkspace and stack stuff up.
	for {
		var ws *Workspace
		ws, searchPath, err = FindWorkspace(fsys, basisPath, searchPath)
		if err != nil {
			return
		}
		if ws == nil {
			break
		}
		wss = append(wss, ws)
	}
	// If no root workspace was found, include the home workspace at the end of the stack.
	// Unless it's already there, of course.
	if len(wss) == 0 || (!wss[len(wss)-1].isHomeWorkspace && !wss[len(wss)-1].IsRootWorkspace()) {
		wss = append(wss, openWorkspace(fsys, homedir))
	}
	return wss, nil
}
