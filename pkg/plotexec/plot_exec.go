package plotexec

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/printer"
	"github.com/warpfork/warpforge/pkg/formulaexec"
	"github.com/warpfork/warpforge/pkg/logging"
	"github.com/warpfork/warpforge/pkg/workspace"
	"github.com/warpfork/warpforge/wfapi"
)

const LOG_TAG_START = "┌─ plot"
const LOG_TAG = "│  plot"
const LOG_TAG_MID = "├─ plot"
const LOG_TAG_END = "└─ plot"

type pipeMap map[wfapi.StepName]map[wfapi.LocalLabel]wfapi.FormulaInput

// Returns a WareID for a given StepName and LocalLabel, if it exists
func (m pipeMap) lookup(stepName wfapi.StepName, label wfapi.LocalLabel) (*wfapi.FormulaInput, error) {
	if step, ok := m[stepName]; ok {
		if input, ok := step[label]; ok {
			// located a valid input
			return &input, nil
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
func plotInputToFormulaInput(wsSet workspace.WorkspaceSet,
	plotInput wfapi.PlotInput,
	pipeCtx pipeMap,
	logger logging.Logger) (wfapi.FormulaInput, *wfapi.WarehouseAddr, error) {
	basis, addr, err := plotInputToFormulaInputSimple(wsSet, plotInput, pipeCtx, logger)
	if err != nil {
		return wfapi.FormulaInput{}, nil, err
	}

	switch {
	case plotInput.PlotInputSimple != nil:
		return wfapi.FormulaInput{
			FormulaInputSimple: &basis,
		}, addr, nil
	case plotInput.PlotInputComplex != nil:
		return wfapi.FormulaInput{
			FormulaInputComplex: &wfapi.FormulaInputComplex{
				Basis:   basis,
				Filters: plotInput.PlotInputComplex.Filters,
			}}, addr, nil
	default:
		panic("unreachable")
	}
}

func plotInputToFormulaInputSimple(wsSet workspace.WorkspaceSet,
	plotInput wfapi.PlotInput,
	pipeCtx pipeMap,
	logger logging.Logger) (wfapi.FormulaInputSimple, *wfapi.WarehouseAddr, error) {
	var basis wfapi.PlotInputSimple

	switch {
	case plotInput.PlotInputSimple != nil:
		basis = *plotInput.PlotInputSimple
	case plotInput.PlotInputComplex != nil:
		basis = plotInput.PlotInputComplex.Basis
	default:
		return wfapi.FormulaInputSimple{}, nil, fmt.Errorf("invalid plot input")
	}

	switch {
	case basis.WareID != nil:
		logger.Info(LOG_TAG, "\t%s = %s\t%s = %s\t%s = %s",
			color.HiBlueString("type"),
			color.WhiteString("ware"),
			color.HiBlueString("wareId"),
			color.WhiteString(basis.WareID.String()),
			color.HiBlueString("packType"),
			color.WhiteString(string(basis.WareID.Packtype)),
		)

		// convert WareID PlotInput to FormulaInput
		return wfapi.FormulaInputSimple{
			WareID: basis.WareID,
		}, nil, nil
	case basis.Mount != nil:
		logger.Info(LOG_TAG, "\t%s = %s\t%s = %s\t%s = %s",
			color.HiBlueString("type"),
			color.WhiteString("mount"),
			color.HiBlueString("hostPath"),
			color.WhiteString(basis.Mount.HostPath),
			color.HiBlueString("mode"),
			color.WhiteString(string(basis.Mount.Mode)),
		)

		// convert WareID PlotInput to FormulaInput
		return wfapi.FormulaInputSimple{
			Mount: basis.Mount,
		}, nil, nil
	case basis.CatalogRef != nil:
		logger.Info(LOG_TAG, "\t%s = %s\n\t\t%s = %s",
			color.HiBlueString("type"),
			color.WhiteString("catalog"),
			color.HiBlueString("ref"),
			color.WhiteString(basis.CatalogRef.String()),
		)

		// find the WareID and WareAddress for this catalog item
		wareId, wareAddr, err := wsSet.GetCatalogWare(*basis.CatalogRef)
		if err != nil {
			return wfapi.FormulaInputSimple{}, nil, err
		}

		if wareId == nil {
			// failed to find a match in the catalog
			return wfapi.FormulaInputSimple{},
				nil,
				fmt.Errorf("no definition found for %q", basis.CatalogRef.String())
		}

		wareStr := "none"
		if wareAddr != nil {
			wareStr = string(*wareAddr)
		}
		logger.Info(LOG_TAG, "\t\t%s = %s\n\t\t%s = %s",
			color.HiBlueString("wareId"),
			color.WhiteString(wareId.String()),
			color.HiBlueString("wareAddr"),
			color.WhiteString(wareStr),
		)

		return wfapi.FormulaInputSimple{
			WareID: wareId,
		}, wareAddr, nil

	case basis.Pipe != nil:
		// resolve the pipe to a WareID using the pipeCtx
		input, err := pipeCtx.lookup(basis.Pipe.StepName, basis.Pipe.Label)
		return *input.Basis(), nil, err

	case basis.Ingest != nil && basis.Ingest.GitIngest != nil:
		input := wfapi.FormulaInputSimple{
			WareID: &wfapi.WareID{},
		}

		path, err := filepath.Abs(basis.Ingest.GitIngest.HostPath)
		if err != nil {
			return wfapi.FormulaInputSimple{}, nil, fmt.Errorf("failed to get absolute path for git ingest")
		}

		// populate cache dir with git ingest
		//
		// note, this executes on the host, not in a container. however, this does work, because it will be checked out
		// and owned by the same user that invokes runc, resulting in all files being owned by uid 0 within the container.
		// this doesn't work for tarballs (which preserve persmissions) but does work for git (which does not).
		//
		// since the cache dir will be populated before formula exec occurs, the rio unpack step will
		// be skipped for this input.
		ws, _ := workspace.OpenHomeWorkspace(os.DirFS("/"))

		// resolve the revision of the git ingest to a hash
		repo, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
			URL: "file://" + path,
		})
		if err != nil {
			return input, nil, fmt.Errorf("failed to checkout git repository at %q to memory: %s", path, err)
		}

		hashBytes, err := repo.ResolveRevision(plumbing.Revision(basis.Ingest.GitIngest.Ref))
		if err != nil {
			return input, nil, fmt.Errorf("failed to resolve git hash: %s", err)
		}

		// create our formula ware id using the resolved hash
		input.WareID.Hash = hashBytes.String()
		input.WareID.Packtype = "git"

		// checkout the git repository to the cache path
		cachePath, err := ws.CachePath(*input.WareID)
		if err != nil {
			return input, nil, err
		}
		if _, err = os.Stat(cachePath); os.IsNotExist(err) {
			_, err = git.PlainClone(cachePath, false, &git.CloneOptions{
				URL:               "file://" + path,
				RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
			})

			if err != nil {
				return input, nil, fmt.Errorf("failed to checkout git ingest: %s", err)
			}
		}
		return input, nil, nil

	}
	return wfapi.FormulaInputSimple{}, nil, fmt.Errorf("invalid type in plot input")
}

func execProtoformula(wsSet workspace.WorkspaceSet,
	pf wfapi.Protoformula,
	ctx wfapi.FormulaContext,
	pipeCtx pipeMap,
	logger logging.Logger) (wfapi.RunRecord, error) {
	// create an empty Formula and FormulaContext
	formula := wfapi.Formula{
		Action: pf.Action,
	}
	formula.Inputs.Values = make(map[wfapi.SandboxPort]wfapi.FormulaInput)
	formula.Outputs.Values = make(map[wfapi.OutputName]wfapi.GatherDirective)

	// convert Protoformula inputs (of type PlotInput) to FormulaInputs
	for sbPort, plotInput := range pf.Inputs.Values {
		formula.Inputs.Keys = append(formula.Inputs.Keys, sbPort)
		input, wareAddr, err := plotInputToFormulaInput(wsSet, plotInput, pipeCtx, logger)
		if err != nil {
			return wfapi.RunRecord{}, err
		}
		formula.Inputs.Values[sbPort] = input
		if wareAddr != nil {
			// input specifies a WarehouseAddr, add it to the formula's context
			ctx.Warehouses.Keys = append(ctx.Warehouses.Keys, *input.Basis().WareID)
			ctx.Warehouses.Values[*input.Basis().WareID] = *wareAddr
		}
	}

	// convert Protoformula outputs to Formula outputs
	for label, gatherDirective := range pf.Outputs.Values {
		label := wfapi.OutputName(label)
		formula.Outputs.Keys = append(formula.Outputs.Keys, label)
		formula.Outputs.Values[label] = gatherDirective
	}

	// execute the derived formula
	rr, err := formulaexec.Exec(wsSet.Root,
		wfapi.FormulaAndContext{
			Formula: formula,
			Context: &ctx,
		},
		logger)
	return rr, err
}

func Exec(wsSet workspace.WorkspaceSet, plot wfapi.Plot, logger logging.Logger) (wfapi.PlotResults, error) {
	pipeCtx := make(pipeMap)
	results := wfapi.PlotResults{}

	logger.Info(LOG_TAG_START, "")

	// convert plot to node

	nPlot := bindnode.Wrap(&plot, wfapi.TypeSystem.TypeByName("Plot"))
	/*
		if err != nil {
			return results, fmt.Errorf("could not wrap plot: %s", err)
		}
	*/
	logger.Debug(LOG_TAG, printer.Sprint(nPlot))

	// collect the plot inputs
	// these have an empty string for the step name (e.g., `pipe::foo`)
	logger.Info(LOG_TAG, "inputs:")
	pipeCtx[""] = make(map[wfapi.LocalLabel]wfapi.FormulaInput)
	inputContext := wfapi.FormulaContext{}
	inputContext.Warehouses.Values = make(map[wfapi.WareID]wfapi.WarehouseAddr)
	for name, input := range plot.Inputs.Values {
		input, wareAddr, err := plotInputToFormulaInput(wsSet, input, pipeCtx, logger)
		if err != nil {
			return results, err
		}
		pipeCtx[""][name] = input
		if wareAddr != nil {
			// input specifies an address, add it to the context
			inputContext.Warehouses.Keys = append(inputContext.Warehouses.Keys, *input.Basis().WareID)
			inputContext.Warehouses.Values[*input.Basis().WareID] = *wareAddr
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
			logger.Info(LOG_TAG_MID, "(%s) %s",
				color.HiCyanString(string(name)),
				color.WhiteString("evaluating protoformula"),
			)
			rr, err := execProtoformula(wsSet, *step.Protoformula, inputContext, pipeCtx, logger)
			if err != nil {
				return results, fmt.Errorf("failed to execute protoformula for step %s: %s", name, err)
			}
			// accumulate the results of the Protoformula our map of Pipes
			pipeCtx[name] = make(map[wfapi.LocalLabel]wfapi.FormulaInput)
			for result, input := range rr.Results.Values {
				logger.Info(LOG_TAG, "(%s) %s %s:%s",
					color.HiCyanString(string(name)),
					color.WhiteString("collected output"),
					color.WhiteString(string(name)), color.WhiteString(string(result)),
				)
				pipeCtx[name][wfapi.LocalLabel(result)] = wfapi.FormulaInput{
					FormulaInputSimple: &input,
				}
			}
		case step.Plot != nil:
			// execute plot step
			logger.Info(LOG_TAG_MID, "(%s) %s",
				color.HiCyanString(string(name)),
				color.WhiteString("evaluating subplot"),
			)

			stepResults, err := Exec(wsSet, *step.Plot, logger)
			if err != nil {
				return results, fmt.Errorf("failed to execute plot for step %s: %s", name, err)
			}
			// accumulate the results of the Plot into our map of Pipes
			pipeCtx[name] = make(map[wfapi.LocalLabel]wfapi.FormulaInput)
			for result, wareId := range stepResults.Values {
				logger.Info(LOG_TAG, "(%s) %s %s:%s",
					color.HiCyanString(string(name)),
					color.WhiteString("collected output"),
					color.WhiteString(string(name)), color.WhiteString(string(result)),
				)

				pipeCtx[name][wfapi.LocalLabel(result)] = wfapi.FormulaInput{
					FormulaInputSimple: &wfapi.FormulaInputSimple{
						WareID: &wareId,
					},
				}
			}
		default:
			return results, fmt.Errorf("invalid step %s", name)
		}

		logger.Info(LOG_TAG_MID, "(%s) %s",
			color.HiCyanString(string(name)),
			color.WhiteString("complete"),
		)
		logger.Info(LOG_TAG, "")
	}

	logger.Info(LOG_TAG, "outputs:")
	// collect the outputs of this plot
	results.Values = make(map[wfapi.LocalLabel]wfapi.WareID)
	for name, output := range plot.Outputs.Values {
		result, err := pipeCtx.lookup(output.Pipe.StepName, output.Pipe.Label)
		if err != nil {
			return results, err
		}
		logger.Info(LOG_TAG, "\t%s -> %s",
			color.HiBlueString(string(name)),
			color.WhiteString(result.Basis().WareID.String()))
		results.Keys = append(results.Keys, name)
		results.Values[name] = *result.Basis().WareID
	}
	logger.Info(LOG_TAG_END, "")
	return results, nil
}
