package plotexec

import (
	"fmt"

	"github.com/warpfork/warpforge/pkg/formulaexec"
	"github.com/warpfork/warpforge/wfapi"
)

type pipeMap map[wfapi.StepName]map[wfapi.LocalLabel]wfapi.FormulaInput

// Returns a FormulaInput for a given StepName and LocalLabel, if it exists
func (m pipeMap) lookup(stepName wfapi.StepName, label wfapi.LocalLabel) (wfapi.FormulaInput, error) {
	if step, ok := m[stepName]; ok {
		if input, ok := step[label]; ok {
			// located a valid input
			return input, nil
		} else {
			// located step, but no input by label
			if stepName == "" {
				return wfapi.FormulaInput{}, fmt.Errorf("no label '%s' in plot inputs ('pipe::%s' not defined)", label, label)
			} else {
				return wfapi.FormulaInput{}, fmt.Errorf("no label '%s' for step '%s' (pipe:%s:%s not defined)", label, stepName, stepName, label)
			}
		}
	} else {
		// did not locate step
		return wfapi.FormulaInput{}, fmt.Errorf("no step '%s'", stepName)
	}
}

func plotInputToFormulaInput(plotInput wfapi.PlotInput, pipeCtx pipeMap) (wfapi.FormulaInput, error) {
	var basis wfapi.PlotInputSimple
	var filters wfapi.FilterMap

	switch {
	case plotInput.PlotInputSimple != nil:
		basis = *plotInput.PlotInputSimple
	case plotInput.PlotInputComplex != nil:
		basis = plotInput.PlotInputComplex.Basis
		filters = plotInput.PlotInputComplex.Filters
	default:
		return wfapi.FormulaInput{}, fmt.Errorf("invalid plot input")
	}

	var simple wfapi.FormulaInputSimple
	switch {
	case basis.WareID != nil:
		simple = wfapi.FormulaInputSimple{
			WareID: basis.WareID,
		}
	/* TODO
	case basis.CatalogRef != nil:
		fmt.Println("WARNING: CatalogRef not implemented, ignoring")
		return wfapi.FormulaInput{}, nil
	*/
	case basis.Pipe != nil:
		return pipeCtx.lookup(basis.Pipe.StepName, basis.Pipe.Label)
	default:
		return wfapi.FormulaInput{}, fmt.Errorf("invalid type in plot input")
	}

	// if filters exist, need to construct a FormulaInputComplex
	// if there are no filters, use a FormulaInputSimple
	if len(filters.Keys) == 0 {
		return wfapi.FormulaInput{
			FormulaInputSimple: &simple,
		}, nil
	} else {
		return wfapi.FormulaInput{
			FormulaInputComplex: &wfapi.FormulaInputComplex{
				Basis:   simple,
				Filters: filters,
			}}, nil
	}
}

func execProtoformula(pf wfapi.Protoformula, pipeCtx pipeMap) (wfapi.RunRecord, error) {
	// create an empty formula
	formula := wfapi.Formula{
		Action: pf.Action,
	}
	formula.Inputs.Values = make(map[wfapi.SandboxPort]wfapi.FormulaInput)
	formula.Outputs.Values = make(map[wfapi.OutputName]wfapi.GatherDirective)

	// convert protoformula inputs (type PlotInput) to FormulaInputs
	for sbPort, plotInput := range pf.Inputs.Values {
		formula.Inputs.Keys = append(formula.Inputs.Keys, sbPort)
		formulaInput, err := plotInputToFormulaInput(plotInput, pipeCtx)
		if err != nil {
			return wfapi.RunRecord{}, err
		}
		formula.Inputs.Values[sbPort] = formulaInput
	}

	// convert protoformula outputs to formula outputs
	for label, gatherDirective := range pf.Outputs.Values {
		label := wfapi.OutputName(label)
		formula.Outputs.Keys = append(formula.Outputs.Keys, label)
		formula.Outputs.Values[label] = gatherDirective
	}

	// execute the derived formula
	rr, err := formulaexec.Exec(wfapi.FormulaAndContext{
		Formula: formula,
		Context: &wfapi.FormulaContext{},
	})
	return rr, err
}

func Exec(plot wfapi.Plot) (wfapi.PlotResults, error) {
	pipeCtx := make(pipeMap)
	results := wfapi.PlotResults{}

	// collect the plot inputs
	// these have an empty string for the step name (e.g., `pipe::foo`)
	pipeCtx[""] = make(map[wfapi.LocalLabel]wfapi.FormulaInput)
	for name, input := range plot.Inputs.Values {
		formulaInput, err := plotInputToFormulaInput(input, pipeCtx)
		if err != nil {
			return results, err
		}
		pipeCtx[""][name] = formulaInput
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
			// execute protoformula step
			rr, err := execProtoformula(*step.Protoformula, pipeCtx)
			if err != nil {
				return results, fmt.Errorf("failed to execute protoformula for step %s: %s", name, err)
			}
			// accumulate the results of the protoformula
			pipeCtx[name] = make(map[wfapi.LocalLabel]wfapi.FormulaInput)
			for result, input := range rr.Results.Values {
				fmt.Println(input.WareID)
				pipeCtx[name][wfapi.LocalLabel(result)] = wfapi.FormulaInput{
					FormulaInputSimple: &input,
				}
			}
		case step.Plot != nil:
			// execute plot step
			stepResults, err := Exec(*step.Plot)
			if err != nil {
				return results, fmt.Errorf("failed to execute plot for step %s: %s", name, err)
			}
			// accumulate the results of the plot
			pipeCtx[name] = make(map[wfapi.LocalLabel]wfapi.FormulaInput)
			for result, input := range stepResults.Values {
				pipeCtx[name][wfapi.LocalLabel(result)] = wfapi.FormulaInput{
					FormulaInputSimple: &wfapi.FormulaInputSimple{
						WareID: &input,
					},
				}
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
		results.Values[name] = *result.FormulaInputSimple.WareID
	}
	return results, nil
}
