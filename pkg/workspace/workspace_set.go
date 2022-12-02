package workspace

import (
	"context"
	"fmt"

	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/wfapi"
)

// A WorkspaceSet is a slice of workspaces ending with a root workspace.
type WorkspaceSet []*Workspace

// Local returns the most local workspace.
// This workspace _MAY_ be a root workspace
func (wsSet WorkspaceSet) Local() *Workspace {
	return wsSet[0]
}

// Root returns the root workspace of the workspace set
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

// Tidy will bundle plot dependencies into the local workspace from any workspace in the set.
// The local workspace must be a non-root workspace
//
// Errors:
//
//    - warpforge-error-catalog-invalid -- a catalog in this workspace set is invalid
//    - warpforge-error-catalog-parse -- a catalog in this workspace set can't be parsed
//    - warpforge-error-missing-catalog-entry -- a dependency can't be found
//    - warpforge-error-catalog-name -- honestly, shouldn't happen
//    - warpforge-error-workspace -- workspace stack missing a non-root workspace
//    - warpforge-error-io -- reading/writing catalog fails
func (wsSet WorkspaceSet) Tidy(ctx context.Context, plot wfapi.Plot, force bool) wfapi.Error {
	local := wsSet.Local()
	if local.IsRootWorkspace() {
		_, path := local.Path()
		return wfapi.ErrorWorkspace(path, fmt.Errorf("workspace stack needs a non-root workspace"))
	}

	refs := gatherCatalogRefs(plot)

	cat, err := local.CreateOrOpenCatalog("")
	if err != nil {
		return err
	}
	for _, ref := range refs {
		wareId, wareAddr, err := wsSet.GetCatalogWare(ref)
		if err != nil {
			return err
		}

		if wareId == nil {
			return wfapi.ErrorMissingCatalogEntry(ref, true) // We don't care whether the replay is available?
		}

		logger := logging.Ctx(ctx)
		logger.Info("", "bundled \"%s:%s:%s\"\n", ref.ModuleName, ref.ReleaseName, ref.ItemName)

		cat.AddItem(ref, *wareId, force)
		if wareAddr != nil {
			err := cat.AddByWareMirror(ref, *wareId, *wareAddr)
			if err != nil {
				logger.Debug("", "error adding ware: %s", err)
			}
		}
	}
	return nil
}

// GetWarehouses returns a set of warehouse addresses for all the warehouses in the workspace stack
func (wsSet WorkspaceSet) GetWarehouseAddresses() []wfapi.WarehouseAddr {
	result := make([]wfapi.WarehouseAddr, 0, len(wsSet))
	for _, ws := range wsSet {
		result = append(result, ws.GetWarehouseAddress())
	}
	return result
}

func gatherCatalogRefs(plot wfapi.Plot) []wfapi.CatalogRef {
	refs := []wfapi.CatalogRef{}

	// gather this plot's inputs
	for _, input := range plot.Inputs.Values {
		if input.Basis().CatalogRef != nil {
			refs = append(refs, *input.Basis().CatalogRef)
		}
	}

	// gather subplot inputs
	for _, step := range plot.Steps.Values {
		if step.Plot != nil {
			// recursively gather the refs from subplot(s)
			newRefs := gatherCatalogRefs(*step.Plot)

			// deduplicate
			unique := true
			for _, newRef := range newRefs {
				for _, existingRef := range refs {
					if newRef == existingRef {
						unique = false
						break
					}
				}
				if unique {
					refs = append(refs, newRef)
				}
			}
		}
	}

	return refs
}
