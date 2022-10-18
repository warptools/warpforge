package workspace

import (
	"github.com/warpfork/warpforge/wfapi"
)

// A WorkspaceSet is a slice of workspaces ending with a root workspace.
type WorkspaceSet []*Workspace

func (wsSet WorkspaceSet) Root() *Workspace {
	return wsSet[len(wsSet)-1]
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
//    - warpforge-error-internal -- when a catalog name is invalid
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
//    - warpforge-error-internal -- when a catalog name is invalid
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
