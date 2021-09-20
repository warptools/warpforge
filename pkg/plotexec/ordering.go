package plotexec

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/goccy/go-graphviz"
	"github.com/goccy/go-graphviz/cgraph"
	"github.com/warpfork/warpforge/wfapi"
)

// Generates a graphviz graph of a given plot, recursing into all subplots.
// Renders the resulting graph in the `format` provided to the `out` buffer.
func Graph(plot wfapi.Plot, format graphviz.Format, out *bytes.Buffer) error {
	// set up graphviz and create an empty graph
	g := graphviz.New()
	graph, err := g.Graph()
	if err != nil {
		return err
	}
	defer func() {
		if err := graph.Close(); err != nil {
			panic("failed to close graphviz graph")
		}
		g.Close()
	}()

	// graph styling
	graph.SetNodeSeparator(0.75)

	// create top level input and output nodes
	inputsTop, err := graph.CreateNode("inputs")
	if err != nil {
		return err
	}
	outputsTop, err := graph.CreateNode("outputs")
	if err != nil {
		return err
	}

	// generate graph
	err = graphPlot(graph, plot, "", inputsTop, outputsTop)
	if err != nil {
		return err
	}

	// render result as requested format to provided buffer
	if err := g.Render(graph, format, out); err != nil {
		return err
	}

	return nil
}

func graphPlot(graph *cgraph.Graph, plot wfapi.Plot, prefix string, inputNode *cgraph.Node, outputNode *cgraph.Node) error {
	// begin with steps in the execution order
	ordered, err := OrderSteps(plot)
	if err != nil {
		return err
	}

	// map to store step graph nodes by name
	nodes := make(map[wfapi.StepName]*cgraph.Node)

	// iterate over ordered steps
	for _, stepName := range ordered {
		step := plot.Steps.Values[stepName]

		// add the current step name as a prefix to the node identifier
		// this is needed to ensure nodes have unique identifiers
		n, err := graph.CreateNode(fmt.Sprintf("%s%s", prefix, stepName))
		if err != nil {
			return err
		}
		nodes[stepName] = n

		// label the node without the prefix
		n.SetLabel(string(stepName))

		// set the colour depending on step type
		if step.Protoformula != nil {
			n.SetColor("red")
		}

		// resolve inputs for this step
		switch {
		case step.Protoformula != nil:
			for _, input := range step.Protoformula.Inputs.Values {
				if input.PlotInputSimple.Pipe != nil {
					var src *cgraph.Node
					if input.PlotInputSimple.Pipe.StepName == "" {
						// empty step name, use the inputNode (e.g., pipe::foo)
						src = inputNode
					} else {
						src = nodes[input.PlotInputSimple.Pipe.StepName]
					}
					name := fmt.Sprintf("pipe:%s:%s", input.PlotInputSimple.Pipe.StepName, input.PlotInputSimple.Pipe.Label)
					e, err := graph.CreateEdge(name, src, n)
					if err != nil {
						return err
					}
					e.SetLabel(name)
				}
			}
		case step.Plot != nil:
			// create a subgraph for the nested plot
			// the "cluster_" prefix tells graphviz to group this graphicall
			subgraph := graph.SubGraph(fmt.Sprintf("cluster_%s", stepName), 1)

			// set styling for subgraph
			subgraph.SetStyle("filled")
			subgraph.SetLabel(string(stepName))
			subgraph.SetLabelLocation(cgraph.BottomLocation)
			subgraph.SetLabelJust(cgraph.RightJust)

			// the plot node will be used as the destination for outputs
			// set the label to "outputs" since it will be in a named subgraph
			n.SetLabel("outputs")

			// create the input node for the nested plot
			plotInputs, err := graph.CreateNode(fmt.Sprintf("%s_inputs", stepName))
			if err != nil {
				return err
			}
			plotInputs.SetLabel("inputs")

			// add nested plot inputs to the input node
			for _, input := range step.Plot.Inputs.Values {
				if input.PlotInputSimple.Pipe != nil {
					var src *cgraph.Node
					if input.PlotInputSimple.Pipe.StepName == "" {
						// empty step name, use the inputNode (e.g., pipe::foo)
						src = inputNode
					} else {
						src = nodes[input.PlotInputSimple.Pipe.StepName]
					}

					name := fmt.Sprintf("pipe:%s:%s", input.PlotInputSimple.Pipe.StepName, input.PlotInputSimple.Pipe.Label)
					e, err := graph.CreateEdge(name, src, plotInputs)
					if err != nil {
						return err
					}
					e.SetLabel(name)
				}
			}

			// recurse into subplot
			err = graphPlot(subgraph, *plot.Steps.Values[stepName].Plot, string(stepName), plotInputs, n)
			if err != nil {
				return err
			}

		default:
			panic("unreachable")
		}

	}

	// create edges for this plot's output
	for _, output := range plot.Outputs.Values {
		name := fmt.Sprintf("pipe:%s:%s", output.Pipe.StepName, output.Pipe.Label)
		e, err := graph.CreateEdge(name, nodes[output.Pipe.StepName], outputNode)
		if err != nil {
			return err
		}
		e.SetLabel(name)
	}

	return nil
}

// Return the ordered list of steps for a plot, recursing into nested plots.
func OrderStepsAll(plot wfapi.Plot) ([]wfapi.StepName, error) {
	var result []wfapi.StepName
	ordered, err := OrderSteps(plot)
	if err != nil {
		return result, err
	}

	for _, step := range ordered {
		// add this step to the results list
		result = append(result, step)
		if plot.Steps.Values[step].Plot != nil {
			// recurse into subplot
			subOrdered, err := OrderStepsAll(*plot.Steps.Values[step].Plot)
			if err != nil {
				return result, err
			}
			result = append(result, subOrdered...)
		}
	}

	return result, nil
}

// Return the ordered list of steps for a single plot.
func OrderSteps(plot wfapi.Plot) ([]wfapi.StepName, error) {
	// initialize results accumulator
	result := make([]wfapi.StepName, 0, len(plot.Steps.Keys))
	// initialize todo list, shrinks as steps are processed
	todo := make(map[wfapi.StepName]struct{}, len(plot.Steps.Keys))
	for _, step := range plot.Steps.Keys {
		todo[step] = struct{}{}
	}

	// begin with steps sorted by name
	stepsOrdered := make([]wfapi.StepName, len(plot.Steps.Keys))
	copy(stepsOrdered, plot.Steps.Keys)
	sort.Sort(stepNamesByLex(stepsOrdered))

	// visit each step
	for _, name := range stepsOrdered {
		err := orderSteps_visit(name, plot.Steps.Values[name], todo, map[wfapi.StepName]struct{}{}, &result, plot)
		if err != nil {
			return []wfapi.StepName{}, err
		}
	}
	return result, nil
}

func orderSteps_visit(
	name wfapi.StepName,
	step wfapi.Step,
	todo map[wfapi.StepName]struct{},
	loopDetector map[wfapi.StepName]struct{},
	result *[]wfapi.StepName,
	plot wfapi.Plot,
) error {

	// if step has already been visited, we're done
	if _, ok := todo[name]; !ok {
		return nil
	}

	// if step is in loop detection, fail
	if _, ok := loopDetector[name]; ok {
		return fmt.Errorf("plot inputs not a DAG: loop detected at step '%s'", name)
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
				return fmt.Errorf("invalid pipe 'pipe::%s', input '%s' does not exist", pipe.Label, pipe.Label)
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
					return fmt.Errorf("invalid pipe 'pipe:%s:%s', label '%s' does not exist for step %s", pipe.StepName, pipe.Label, pipe.Label, pipe.StepName)
				}
			} else {
				return fmt.Errorf("invalid pipe 'pipe:%s:%s', step '%s' does not exist", pipe.StepName, pipe.Label, pipe.StepName)
			}
		}
	}

	// sort pipes by name (to ensure determinism), then recurse
	sort.Sort(pipesByLex(inputPipes))
	for _, pipe := range inputPipes {
		switch pipe.StepName == "" {
		case true:
			// top level input, nothing to do
		case false:
			// recurse the referenced step
			if err := orderSteps_visit(pipe.StepName, plot.Steps.Values[pipe.StepName], todo, loopDetector, result, plot); err != nil {
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
