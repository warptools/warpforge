package plotexec

import (
	"fmt"

	"github.com/warpfork/warpforge/pkg/formulaexec"
	"github.com/warpfork/warpforge/pkg/workspace"
	"github.com/warpfork/warpforge/wfapi"
)

type pipeMap map[wfapi.StepName]map[wfapi.LocalLabel]wfapi.WareID

// Returns a WareID for a given StepName and LocalLabel, if it exists
func (m pipeMap) lookup(stepName wfapi.StepName, label wfapi.LocalLabel) (*wfapi.WareID, error) {
	if step, ok := m[stepName]; ok {
		if ware, ok := step[label]; ok {
			// located a valid input
			return &ware, nil
		} else {
			// located step, but no input by label
			if stepName == "" {
				return nil, fmt.Errorf("no label '%s' in plot inputs ('pipe::%s' not defined)", label, label)
			} else {
				return nil, fmt.Errorf("no label '%s' for step '%s' (pipe:%s:%s not defined)", label, stepName, stepName, label)
			}
		}
	} else {
		// did not locate step
		return nil, fmt.Errorf("no step '%s'", stepName)
	}
}

// Resolves a PlotInput to a WareID and optionally a WarehouseAddr.
// This will resolve various input types (Pipes, CatalogRefs, etc...)
// to allow them to be used in a Formula.
func plotInputToWare(wss []*workspace.Workspace, plotInput wfapi.PlotInput, pipeCtx pipeMap) (*wfapi.WareID, *wfapi.WarehouseAddr, error) {
	var basis wfapi.PlotInputSimple

	switch {
	case plotInput.PlotInputSimple != nil:
		basis = *plotInput.PlotInputSimple
	case plotInput.PlotInputComplex != nil:
		basis = plotInput.PlotInputComplex.Basis
	default:
		return nil, nil, fmt.Errorf("invalid plot input")
	}

	switch {
	case basis.WareID != nil:
		// if it's just a WareID, we're done
		return basis.WareID, nil, nil
	case basis.CatalogRef != nil:
		// search the warehouse stack for this CatalogRef
		// this will return the WareID and WarehouseAddr to use
		var wareId *wfapi.WareID
		for _, ws := range wss {
			wareId, wareAddr, err := ws.GetCatalogWare(*basis.CatalogRef)
			if err != nil {
				return nil, nil, err
			}
			if wareId != nil {
				// found a matching ware in a catalog, stop searching
				return wareId, wareAddr, nil
			}
		}
		if wareId == nil {
			// failed to find a match in the catalog
			return nil, nil, fmt.Errorf("no definition found for %q", basis.CatalogRef.String())
		}
	case basis.Pipe != nil:
		// resolve the pipe to a WareID using the pipeCtx
		wareId, err := pipeCtx.lookup(basis.Pipe.StepName, basis.Pipe.Label)
		return wareId, nil, err
	}

	// if filters exist, need to construct a FormulaInputComplex
	// if there are no filters, use a FormulaInputSimple
	/* TODO move this elsewhere!
	if len(filters.Keys) == 0 {
		return wfapi.FormulaInput{
			FormulaInputSimple: simple,
		}, nil
	} else {
		return wfapi.FormulaInput{
			FormulaInputComplex: &wfapi.FormulaInputComplex{
				Basis:   *simple,
				Filters: filters,
			}}, nil
	}
	*/
	return nil, nil, fmt.Errorf("invalid type in plot input")
}

func execProtoformula(wss []*workspace.Workspace, pf wfapi.Protoformula, ctx wfapi.FormulaContext, pipeCtx pipeMap) (wfapi.RunRecord, error) {
	// create an empty Formula and FormulaContext
	formula := wfapi.Formula{
		Action: pf.Action,
	}
	formula.Inputs.Values = make(map[wfapi.SandboxPort]wfapi.FormulaInput)
	formula.Outputs.Values = make(map[wfapi.OutputName]wfapi.GatherDirective)

	// get the home workspace from the workspace stack
	var homeWs *workspace.Workspace
	for _, ws := range wss {
		if ws.IsHomeWorkspace() {
			homeWs = ws
			break
		}
	}

	// convert Protoformula inputs (of type PlotInput) to FormulaInputs
	for sbPort, plotInput := range pf.Inputs.Values {
		formula.Inputs.Keys = append(formula.Inputs.Keys, sbPort)
		wareId, wareAddr, err := plotInputToWare(wss, plotInput, pipeCtx)
		if err != nil {
			return wfapi.RunRecord{}, err
		}
		formula.Inputs.Values[sbPort] = wfapi.FormulaInput{
			FormulaInputSimple: &wfapi.FormulaInputSimple{
				WareID: wareId,
			},
		}
		if wareAddr != nil {
			// input specifies a WarehouseAddr for this WareID
			// add it to the formula's context
			ctx.Warehouses.Keys = append(ctx.Warehouses.Keys, *wareId)
			ctx.Warehouses.Values[*wareId] = *wareAddr
		}
	}

	// convert Protoformula outputs to Formula outputs
	for label, gatherDirective := range pf.Outputs.Values {
		label := wfapi.OutputName(label)
		formula.Outputs.Keys = append(formula.Outputs.Keys, label)
		formula.Outputs.Values[label] = gatherDirective
	}

	// execute the derived formula
	rr, err := formulaexec.Exec(homeWs, wfapi.FormulaAndContext{
		Formula: formula,
		Context: &ctx,
	})
	return rr, err
}

func Exec(wss []*workspace.Workspace, plot wfapi.Plot) (wfapi.PlotResults, error) {
	pipeCtx := make(pipeMap)
	results := wfapi.PlotResults{}

	// collect the plot inputs
	// these have an empty string for the step name (e.g., `pipe::foo`)
	pipeCtx[""] = make(map[wfapi.LocalLabel]wfapi.WareID)
	inputContext := wfapi.FormulaContext{}
	inputContext.Warehouses.Values = make(map[wfapi.WareID]wfapi.WarehouseAddr)
	for name, input := range plot.Inputs.Values {
		wareId, wareAddr, err := plotInputToWare(wss, input, pipeCtx)
		if err != nil {
			return results, err
		}
		pipeCtx[""][name] = *wareId
		if wareAddr != nil {
			// input specifies an address, add it to the context
			inputContext.Warehouses.Keys = append(inputContext.Warehouses.Keys, *wareId)
			inputContext.Warehouses.Values[*wareId] = *wareAddr
		}
	}

	// determine step execution order
	stepsOrdered, err := OrderSteps(plot)
	if err != nil {
		return results, err
	}

	// execute the plot steps
	for _, name := range stepsOrdered {
		step := plot.Steps.Values[name]
		switch {
		case step.Protoformula != nil:
			// execute Protoformula step
			rr, err := execProtoformula(wss, *step.Protoformula, inputContext, pipeCtx)
			if err != nil {
				return results, fmt.Errorf("failed to execute protoformula for step %s: %s", name, err)
			}
			// accumulate the results of the Protoformula our map of Pipes
			pipeCtx[name] = make(map[wfapi.LocalLabel]wfapi.WareID)
			for result, input := range rr.Results.Values {
				pipeCtx[name][wfapi.LocalLabel(result)] = *input.WareID
			}
		case step.Plot != nil:
			// execute plot step
			stepResults, err := Exec(wss, *step.Plot)
			if err != nil {
				return results, fmt.Errorf("failed to execute plot for step %s: %s", name, err)
			}
			// accumulate the results of the Plot into our map of Pipes
			pipeCtx[name] = make(map[wfapi.LocalLabel]wfapi.WareID)
			for result, input := range stepResults.Values {
				pipeCtx[name][wfapi.LocalLabel(result)] = input
			}
		default:
			return results, fmt.Errorf("invalid step %s", name)
		}
	}

	// collect the outputs of this plot
	results.Values = make(map[wfapi.LocalLabel]wfapi.WareID)
	for name, output := range plot.Outputs.Values {
		result, err := pipeCtx.lookup(output.Pipe.StepName, output.Pipe.Label)
		if err != nil {
			return results, err
		}
		results.Keys = append(results.Keys, name)
		results.Values[name] = *result
	}
	return results, nil
}
