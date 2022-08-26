package plotexec

import (
	"github.com/warpfork/warpforge/pkg/workspace"
	"github.com/warpfork/warpforge/wfapi"
)

// Might not match the package name -- funcs in this file certainly don't exec anything.

type PlotStats struct {
	InputsUsingCatalog      int
	InputsUsingIngest       int
	InputsUsingMount        int
	ResolvableCatalogInputs int
	ResolvedCatalogInputs   map[wfapi.CatalogRef]wfapi.WareID // might as well remember it if we already did all that work.
	UnresolvedCatalogInputs map[wfapi.CatalogRef]struct{}
}

// ComputeStats counts up how many times a plot uses various features,
// and also checks for reference resolvablity.
func ComputeStats(plot wfapi.Plot, wsSet workspace.WorkspaceSet) (PlotStats, error) {
	v := PlotStats{
		ResolvedCatalogInputs:   make(map[wfapi.CatalogRef]wfapi.WareID),
		UnresolvedCatalogInputs: make(map[wfapi.CatalogRef]struct{}),
	}
	for _, input := range plot.Inputs.Values {
		inputBasis := input.Basis() // unwrap if it's a complex filtered thing.
		switch {
		// This switch should be exhaustive on the possible members of PlotInputSimple.
		case inputBasis.WareID != nil:
			// not interesting :)
		case inputBasis.Mount != nil:
			v.InputsUsingMount++
		case inputBasis.Literal != nil:
			// not interesting :)
		case inputBasis.Pipe != nil:
			// not interesting :)
		case inputBasis.CatalogRef != nil:
			v.InputsUsingCatalog++
			ware, _, err := wsSet.GetCatalogWare(*inputBasis.CatalogRef)
			if err != nil {
				return v, err // These mean catalog read failed entirely, so we're in deep water.
			}
			if ware == nil {
				v.UnresolvedCatalogInputs[*inputBasis.CatalogRef] = struct{}{}
			} else {
				v.ResolvableCatalogInputs++
				v.ResolvedCatalogInputs[*inputBasis.CatalogRef] = *ware
			}
		case inputBasis.Ingest != nil:
			v.InputsUsingIngest++
		default:
			panic("unreachable")
		}
	}
	return v, nil
}
