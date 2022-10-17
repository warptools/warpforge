package plotexec

import (
	"context"
	"fmt"
	"sort"

	"github.com/warpfork/warpforge/pkg/tracing"
	"github.com/warpfork/warpforge/wfapi"
	"go.opentelemetry.io/otel/trace"
)

// Return the ordered list of steps for a plot, recursing into nested plots.
//
// Errors:
//
//    - warpforge-error-plot-invalid -- when the provided plot is malformed
func OrderStepsAll(ctx context.Context, plot wfapi.Plot) ([]wfapi.StepName, error) {
	ctx, span := tracing.Start(ctx, "OrderStepsAll")
	defer span.End()
	var result []wfapi.StepName
	ordered, err := OrderSteps(ctx, plot)
	if err != nil {
		return result, err
	}

	for _, step := range ordered {
		// add this step to the results list
		result = append(result, step)
		if plot.Steps.Values[step].Plot != nil {
			// recurse into subplot
			subOrdered, err := OrderStepsAll(ctx, *plot.Steps.Values[step].Plot)
			if err != nil {
				return result, err
			}
			result = append(result, subOrdered...)
		}
	}

	return result, nil
}

// Return the ordered list of steps for a single plot.
// Errors:
//
//    - warpforge-error-plot-invalid -- when the plot is not a DAG
func OrderSteps(ctx context.Context, plot wfapi.Plot) ([]wfapi.StepName, error) {
	ctx, span := tracing.Start(ctx, "OrderSteps")
	defer span.End()
	// initialize results accumulator
	result := make([]wfapi.StepName, 0, len(plot.Steps.Keys))
	// initialize todo list, shrinks as steps are processed
	todo := make(map[wfapi.StepName]struct{}, len(plot.Steps.Keys))
	for _, step := range plot.Steps.Keys {
		todo[step] = struct{}{}
	}

	// initialize a map to hold the output pipes of each step
	// this is needed to ensure the top level plot outputs can be resolved
	outputPipes := make(map[wfapi.StepName][]wfapi.LocalLabel)

	// begin with steps sorted by name
	stepsOrdered := make([]wfapi.StepName, len(plot.Steps.Keys))
	copy(stepsOrdered, plot.Steps.Keys)
	sort.Sort(stepNamesByLex(stepsOrdered))

	// visit each step
	for _, name := range stepsOrdered {
		err := orderSteps_visit(ctx, name, plot.Steps.Values[name], todo, map[wfapi.StepName]struct{}{}, &result, plot, &outputPipes)
		if err != nil {
			return []wfapi.StepName{}, err
		}
	}

	for _, output := range plot.Outputs.Values {
		// check StepName exists
		stepOutputs, ok := outputPipes[output.Pipe.StepName]
		if !ok {
			return []wfapi.StepName{}, wfapi.ErrorPlotInvalid(fmt.Sprintf("could not resolve plot outputs: no step named %q", output.Pipe.StepName))
		}

		labelFound := false
		for _, label := range stepOutputs {
			if output.Pipe.Label == label {
				labelFound = true
				break
			}
		}
		if !labelFound {
			return []wfapi.StepName{}, wfapi.ErrorPlotInvalid(fmt.Sprintf("could not resolve plot outputs: no output %q in step %q", output.Pipe.Label, output.Pipe.StepName))
		}
	}

	return result, nil
}

// Visit a step of the plot to perform ordering
//
// Errors:
//
//    - warpforge-error-plot-invalid -- when the plot is not a DAG
func orderSteps_visit(
	ctx context.Context,
	name wfapi.StepName,
	step wfapi.Step,
	todo map[wfapi.StepName]struct{},
	loopDetector map[wfapi.StepName]struct{},
	result *[]wfapi.StepName,
	plot wfapi.Plot,
	outputPipes *map[wfapi.StepName][]wfapi.LocalLabel,
) error {
	ctx, span := tracing.Start(ctx, "OrderSteps_visit", trace.WithAttributes(tracing.PrintableAttribute(tracing.AttrKeyWarpforgeStepName, string(name))))
	defer span.End()
	// if step has already been visited, we're done
	if _, ok := todo[name]; !ok {
		return nil
	}

	// if step is in loop detection, fail
	if _, ok := loopDetector[name]; ok {
		return wfapi.ErrorPlotInvalid(fmt.Sprintf("plot inputs not a DAG: loop detected at step '%s'", name))
	}
	// mark step for loop detection
	loopDetector[name] = struct{}{}

	// obtain all input pipes
	stepInputs := []wfapi.PlotInput{}
	switch {
	case step.Protoformula != nil:
		for _, i := range step.Protoformula.Inputs.Values {
			stepInputs = append(stepInputs, i)
		}
	case step.Plot != nil:
		for _, i := range step.Plot.Inputs.Values {
			stepInputs = append(stepInputs, i)
		}
	default:
		panic("unreachable")
	}
	inputPipes := []wfapi.Pipe{}
	for _, i := range stepInputs {
		if i.PlotInputSimple.Pipe != nil {
			inputPipes = append(inputPipes, *i.PlotInputSimple.Pipe)

		} else if i.PlotInputComplex != nil && i.PlotInputComplex.Basis.Pipe != nil {
			inputPipes = append(inputPipes, *i.PlotInputComplex.Basis.Pipe)
		}
	}

	// ensure all pipes can be resolved
	for _, pipe := range inputPipes {
		if pipe.StepName == "" {
			// plot inputs, check input list
			if !labelInList(plot.Inputs.Keys, pipe.Label) {
				return wfapi.ErrorPlotInvalid(fmt.Sprintf("invalid pipe 'pipe::%s', input '%s' does not exist", pipe.Label, pipe.Label))
			}
		} else {
			// handle step pipes
			if s, ok := plot.Steps.Values[pipe.StepName]; ok {
				outputs := []wfapi.LocalLabel{}
				switch {
				case s.Protoformula != nil:
					outputs = append(outputs, s.Protoformula.Outputs.Keys...)
				case s.Plot != nil:
					outputs = append(outputs, s.Plot.Outputs.Keys...)
				default:
					panic("unreachable")
				}
				if !labelInList(outputs, pipe.Label) {
					return wfapi.ErrorPlotInvalid(fmt.Sprintf("invalid pipe 'pipe:%s:%s', label '%s' does not exist for step %s", pipe.StepName, pipe.Label, pipe.Label, pipe.StepName))
				}
			} else {
				return wfapi.ErrorPlotInvalid(fmt.Sprintf("invalid pipe 'pipe:%s:%s', step '%s' does not exist", pipe.StepName, pipe.Label, pipe.StepName))
			}
		}
	}

	// obtain all step outputs
	stepOutputs := []wfapi.LocalLabel{}
	switch {
	case step.Protoformula != nil:
		for label := range step.Protoformula.Outputs.Values {
			stepOutputs = append(stepOutputs, label)
		}
	case step.Plot != nil:
		for label := range step.Plot.Outputs.Values {
			stepOutputs = append(stepOutputs, label)
		}
	default:
		panic("unreachable")
	}

	// place the outputs of this step into the outputPipes map
	for _, label := range stepOutputs {
		(*outputPipes)[name] = append((*outputPipes)[name], label)
	}

	// sort pipes by name (to ensure determinism), then recurse
	sort.Sort(pipesByLex(inputPipes))
	for _, pipe := range inputPipes {
		switch pipe.StepName == "" {
		case true:
			// top level input, nothing to do
		case false:
			// recurse the referenced step
			if err := orderSteps_visit(ctx, pipe.StepName, plot.Steps.Values[pipe.StepName], todo, loopDetector, result, plot, outputPipes); err != nil {
				return err
			}
		}
	}

	// done. put this step in the results, remove from todo
	*result = append(*result, name)
	delete(todo, name)
	return nil
}

func labelInList(ls []wfapi.LocalLabel, l wfapi.LocalLabel) bool {
	for _, v := range ls {
		if v == l {
			return true
		}
	}
	return false
}

type stepNamesByLex []wfapi.StepName

func (a stepNamesByLex) Len() int           { return len(a) }
func (a stepNamesByLex) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a stepNamesByLex) Less(i, j int) bool { return a[i] < a[j] }

type pipesByLex []wfapi.Pipe

func (a pipesByLex) Len() int      { return len(a) }
func (a pipesByLex) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a pipesByLex) Less(i, j int) bool {
	return a[i].StepName < a[j].StepName ||
		a[i].Label < a[j].Label
}
