package workspace

import (
	"io/fs"

	"github.com/warpfork/warpforge/wfapi"
)

// A WorkspaceSet is a slice of workspaces ending with a root workspace.
type WorkspaceSet []*Workspace

func (wsSet WorkspaceSet) Root() *Workspace {
	return wsSet[len(wsSet)-1]
}

// Opens a full WorkspaceSet
// searches from searchPath up to basisPath for workspaces
// The root workspace will be the first workspace found that is marked as a root, or the home workspace if none exists
// Errors:
//
//    - warpforge-error-workspace -- when the workspace directory fails to open
//    - warpforge-error-searching-filesystem -- when an error occurs while searching for the workspace
func OpenWorkspaceSet(fsys fs.FS, basisPath string, searchPath string) (WorkspaceSet, wfapi.Error) {
	set, err := FindWorkspaceStack(fsys, basisPath, searchPath)
	if err != nil {
		return nil, err
	}
	if len(set) == 0 {
		homeWS, err := OpenHomeWorkspace(fsys)
		if err != nil {
			return nil, err
		}
		set = append(set, homeWS)
	}
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
func (wsSet WorkspaceSet) GetCatalogWare(ref wfapi.CatalogRef) (*wfapi.WareID, *wfapi.WarehouseAddr, wfapi.Error) {
	// traverse workspace stack
	for _, ws := range wsSet {
		wareId, wareAddr, err := ws.GetCatalogWare(ref)
		if err != nil {
			return nil, nil, err
		}
		if wareId != nil {
			return wareId, wareAddr, nil
		}
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
func (wsSet WorkspaceSet) GetCatalogReplay(ref wfapi.CatalogRef) (*wfapi.Plot, wfapi.Error) {
	// traverse workspace stack
	for _, ws := range wsSet {
		replay, err := ws.GetCatalogReplay(ref)
		if err != nil {
			return nil, err
		}
		if replay != nil {
			return replay, nil
		}
	}

	return nil, nil
}
