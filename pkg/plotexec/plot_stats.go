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
//
// Any errors arising from this process have to do with failure to load
// catalog info, and causes immediate abort which will result in
// incomplete counts for all features.
//
// Errors:
//
// 	- warpforge-error-catalog-invalid -- like it says on the tin.
// 	- warpforge-error-catalog-parse -- like it says on the tin.
// 	- warpforge-error-io -- for IO errors while reading catalogs.
//
func ComputeStats(plot wfapi.Plot, wsSet workspace.WorkspaceSet) (PlotStats, error) {
	v := PlotStats{
		ResolvedCatalogInputs:   make(map[wfapi.CatalogRef]wfapi.WareID),
		UnresolvedCatalogInputs: make(map[wfapi.CatalogRef]struct{}),
	}
	return v, v.computeStats(plot, wsSet)
}

func (v *PlotStats) computeStats(plot wfapi.Plot, wsSet workspace.WorkspaceSet) error {
	for _, input := range plot.Inputs.Values {
		if err := v.accountForInput(input, wsSet); err != nil {
			return err
		}
	}
	for _, step := range plot.Steps.Values {
		switch {
		case step.Plot != nil:
			if err := v.computeStats(*step.Plot, wsSet); err != nil {
				return err
			}
		case step.Protoformula != nil:
			for _, input := range step.Protoformula.Inputs.Values {
				if err := v.accountForInput(input, wsSet); err != nil {
					return err
				}
			}
		default:
			panic("unreachable")
		}
	}
	return nil
}

func (v *PlotStats) accountForInput(input wfapi.PlotInput, wsSet workspace.WorkspaceSet) error {
	inputBasis := input.Basis() // unwrap if it's a complex filtered thing.
	switch {
	// This switch should be exhaustive on the possible members of PlotInputSimple.
	case inputBasis.WareID != nil:
		return nil // not interesting :)
	case inputBasis.Mount != nil:
		v.InputsUsingMount++
		return nil
	case inputBasis.Literal != nil:
		return nil // not interesting :)
	case inputBasis.Pipe != nil:
		return nil // not interesting :)
	case inputBasis.CatalogRef != nil:
		v.InputsUsingCatalog++
		ware, _, err := wsSet.GetCatalogWare(*inputBasis.CatalogRef)
		if err != nil {
			return err // These mean catalog read failed entirely, so we're in deep water.
		}
		if ware == nil {
			v.UnresolvedCatalogInputs[*inputBasis.CatalogRef] = struct{}{}
		} else {
			v.ResolvableCatalogInputs++
			v.ResolvedCatalogInputs[*inputBasis.CatalogRef] = *ware
		}
		return nil
	case inputBasis.Ingest != nil:
		v.InputsUsingIngest++
		return nil
	default:
		panic("unreachable")
	}
}
