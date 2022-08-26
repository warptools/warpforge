package workspace

import (
	"io/fs"

	"github.com/warpfork/warpforge/wfapi"
)

// a workspace set consists of the 3 types of workspace we operate on
//  home: a workspace containing configuration information and other global info
//  root: a workspace containing catalogs to use, which also stores wares and cache
//        the home workspace is the default root workspace
//  stack: a set of workspaces that may contain additional catalogs and other project-specific info
type WorkspaceSet struct {
	Home  *Workspace
	Root  *Workspace
	Stack []*Workspace // the 0'th index is the closest workspace; the next is its parent, and so on.
}

// Opens a full WorkspaceSet
// searches from searchPath up to basisPath for workspaces
// root workspace will be the first workspace found that is marked as a root, or the home workspace if none exists
// Errors:
//
//    - warpforge-error-workspace -- when the workspace directory fails to open
//    - warpforge-error-searching-filesystem -- when an error occurs while searching for the workspace
func OpenWorkspaceSet(fsys fs.FS, basisPath string, searchPath string) (WorkspaceSet, wfapi.Error) {
	set := WorkspaceSet{}
	home, err := OpenHomeWorkspace(fsys)
	if err != nil {
		// if this failed, continue with no home workspace
		home = nil
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

// Get a catalog ware from a workspace set.
// Looks up a ware by CatalogRef, traversing the workspace set:
//  1. traverses the workspace stack looking in "catalog" dirs.
//  2. looks through all catalogs (within the "catalogs" dir) of the root workspace
//     in alphabetical order, picking the first matching ware found.
//
// Errors:
//
//     - warpforge-error-io -- when an IO error occurs while reading the catalog entry
//     - warpforge-error-catalog-parse -- when ipld parsing of a catalog entry fails
//     - warpforge-error-catalog-invalid -- when ipld parsing of lineage or mirror files fails
func (wsSet *WorkspaceSet) GetCatalogWare(ref wfapi.CatalogRef) (*wfapi.WareID, *wfapi.WarehouseAddr, wfapi.Error) {
	// traverse workspace stack
	for _, ws := range wsSet.Stack {
		wareId, wareAddr, err := ws.GetCatalogWare(ref)
		if err != nil {
			return nil, nil, err
		}
		if wareId != nil {
			return wareId, wareAddr, nil
		}
	}

	// search root workspace
	wareId, wareAddr, err := wsSet.Root.GetCatalogWare(ref)
	if err != nil {
		return nil, nil, err
	}
	if wareId != nil {
		return wareId, wareAddr, nil
	}

	return nil, nil, nil
}

// Get a catalog replay from a workspace set.
// Looks up a ware by CatalogRef, traversing the workspace set:
//  1. traverses the workspace stack looking in "catalog" dirs.
//  2. looks through all catalogs (within the "catalogs" dir) of the root workspace
//     in alphabetical order, picking the first matching ware found.
//
// Errors:
//
//     - warpforge-error-io -- when an IO error occurs while reading the catalog entry
//     - warpforge-error-catalog-parse -- when ipld parsing of a catalog entry fails
//     - warpforge-error-catalog-invalid -- when ipld parsing of lineage or mirror files fails
func (wsSet *WorkspaceSet) GetCatalogReplay(ref wfapi.CatalogRef) (*wfapi.Plot, wfapi.Error) {
	// traverse workspace stack
	for _, ws := range wsSet.Stack {
		replay, err := ws.GetCatalogReplay(ref)
		if err != nil {
			return nil, err
		}
		if replay != nil {
			return replay, nil
		}
	}

	// search root workspace
	return wsSet.Root.GetCatalogReplay(ref)
}
