package plotexec

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/serum-errors/go-serum"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/warptools/warpforge/pkg/formulaexec"
	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/pkg/tracing"
	"github.com/warptools/warpforge/pkg/workspace"
	"github.com/warptools/warpforge/wfapi"
)

const (
	LOG_TAG_START = "┌─ plot"
	LOG_TAG       = "│  plot"
	LOG_TAG_MID   = "├─ plot"
	LOG_TAG_END   = "└─ plot"
)

type pipeMap map[wfapi.StepName]map[wfapi.LocalLabel]wfapi.FormulaInput

type ExecConfig formulaexec.ExecConfig

func (cfg *ExecConfig) debug(ctx context.Context) {
	logger := logging.Ctx(ctx)
	logger.Debug(LOG_TAG, "bin path: %q", cfg.BinPath)
	logger.Debug(LOG_TAG, "run path base: %q", cfg.RunPathBase)
	logger.Debug(LOG_TAG, "keep run dir: %t", cfg.KeepRunDir)
	if cfg.WhPathOverride == nil {
		logger.Debug(LOG_TAG, "warehouse override path: %v", cfg.WhPathOverride)
	} else {
		logger.Debug(LOG_TAG, "warehouse override path: %q", *cfg.WhPathOverride)
	}
}

// Returns a WareID for a given StepName and LocalLabel, if it exists
//
// Errors:
//
//    - warpforge-error-plot-invalid -- when the requested step does not exist
func (m pipeMap) lookup(stepName wfapi.StepName, label wfapi.LocalLabel) (*wfapi.FormulaInput, error) {
	if step, ok := m[stepName]; ok {
		if input, ok := step[label]; ok {
			// located a valid input
			return &input, nil
		} else {
			// located step, but no input by label
			if stepName == "" {
				return nil, wfapi.ErrorPlotInvalid(fmt.Sprintf("no label '%s' in plot inputs ('pipe::%s' not defined)", label, label))
			} else {
				return nil, wfapi.ErrorPlotInvalid(fmt.Sprintf("no label '%s' for step '%s' (pipe:%s:%s not defined)", label, stepName, stepName, label))
			}
		}
	} else {
		// did not locate step
		return nil, wfapi.ErrorPlotInvalid(fmt.Sprintf("step %s was expected, but missing from plot", stepName))
	}
}

// Resolves a PlotInput to a WareID and optionally a WarehouseAddr.
// This will resolve various input types (Pipes, CatalogRefs, etc...)
// to allow them to be used in a Formula.
//
// Errors:
//
//    - warpforge-error-plot-invalid -- when the provided plot input is invalid
//    - warpforge-error-catalog-missing-entry -- when a referenced catalog reference cannot be found
//    - warpforge-error-git -- when a git related error occurs during a git ingest
//    - warpforge-error-io -- when an IO error occurs during conversion
//    - warpforge-error-catalog-parse -- when parsing of catalog files fails
//    - warpforge-error-catalog-invalid -- when the catalog contains invalid data
//    - warpforge-error-plot-step-failed -- when a replay fails
//    - warpforge-error-workspace-missing -- when home workspace is missing or cannot open
func plotInputToFormulaInput(ctx context.Context,
	cfg ExecConfig,
	wss workspace.WorkspaceSet,
	plotInput wfapi.PlotInput,
	plotConfig wfapi.PlotExecConfig,
	pipeCtx pipeMap) (wfapi.FormulaInput, []wfapi.WarehouseAddr, error) {
	ctx, span := tracing.Start(ctx, "plotInputToFormulaInput")
	defer span.End()

	basis, addrs, err := plotInputToFormulaInputSimple(ctx, cfg, wss, plotInput, plotConfig, pipeCtx)
	if err != nil {
		return wfapi.FormulaInput{}, nil, err
	}

	switch {
	case plotInput.PlotInputSimple != nil:
		return wfapi.FormulaInput{
			FormulaInputSimple: &basis,
		}, addrs, nil
	case plotInput.PlotInputComplex != nil:
		return wfapi.FormulaInput{
			FormulaInputComplex: &wfapi.FormulaInputComplex{
				Basis:   basis,
				Filters: plotInput.PlotInputComplex.Filters,
			}}, addrs, nil
	default:
		panic("unreachable")
	}
}

// Converts a plot input into a FormulaInputSimple
//
// Errors:
//
//    - warpforge-error-plot-invalid -- when the provided plot input is invalid
//    - warpforge-error-catalog-missing-entry -- when a referenced catalog reference cannot be found
//    - warpforge-error-git -- when a git related error occurs during a git ingest
//    - warpforge-error-io -- when an IO error occurs during conversion
//    - warpforge-error-catalog-parse -- when parsing of catalog files fails
//    - warpforge-error-catalog-invalid -- when the catalog contains invalid data
//    - warpforge-error-plot-step-failed -- when a replay fails
//    - warpforge-error-workspace-missing -- when home workspace is missing or cannot open
func plotInputToFormulaInputSimple(ctx context.Context,
	cfg ExecConfig,
	wss workspace.WorkspaceSet,
	plotInput wfapi.PlotInput,
	plotCfg wfapi.PlotExecConfig,
	pipeCtx pipeMap) (wfapi.FormulaInputSimple, []wfapi.WarehouseAddr, error) {
	ctx, span := tracing.Start(ctx, "plotInputToFormulaInputSimple")
	defer span.End()
	logger := logging.Ctx(ctx)

	var basis wfapi.PlotInputSimple

	switch {
	case plotInput.PlotInputSimple != nil:
		basis = *plotInput.PlotInputSimple
	case plotInput.PlotInputComplex != nil:
		basis = plotInput.PlotInputComplex.Basis
	default:
		return wfapi.FormulaInputSimple{}, nil,
			wfapi.ErrorPlotInvalid("plot contains input that is neither PlotInputSimple or PlotInputComplex")
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

		// convert mount PlotInput to FormulaInput
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
		wareId, wareAddrs, err := wss.GetCatalogWare(*basis.CatalogRef)
		if err != nil {
			return wfapi.FormulaInputSimple{}, nil, serum.Error(wfapi.ECodeCatalogMissingEntry,
				serum.WithMessageTemplate("could not find {{ catalogRef | q}}"),
				serum.WithDetail("catalogRef", basis.CatalogRef.String()),
				serum.WithCause(err),
			)
		}

		if wareId == nil {
			logger.Debug(LOG_TAG, "failed to resolve catalog reference to ware ID: %s", basis.CatalogRef.String())
			// failed to find a match in the catalog
			return wfapi.FormulaInputSimple{},
				nil,
				wfapi.ErrorMissingCatalogEntry(*basis.CatalogRef, false)
		}
		logger.Info(LOG_TAG, "\t\t%s = %s\n\t\t%s = %s",
			color.HiBlueString("wareId"),
			color.WhiteString(wareId.String()),
			color.HiBlueString("numWarehouses"),
			color.WhiteString("%d", len(wareAddrs)),
		)

		// resolve the replay
		// TODO: unclear if this should happen here or elsewhere
		if len(wareAddrs) == 0 {
			// check if the ware is already in the warehouse
			root := wss.Root()
			warehousePath := filepath.Join("/",
				root.WarehousePath(),
				wareId.Hash[0:3], wareId.Hash[3:6], wareId.Hash)
			_, err := os.Stat(filepath.Join("/", warehousePath))
			if err == nil || !os.IsNotExist(err) {
				return wfapi.FormulaInputSimple{WareID: wareId}, nil, nil
			}
			if err := execReplay(ctx, wss, *basis.CatalogRef, cfg, plotCfg, *wareId); err != nil {
				// ware not found, run the replay to generate it
				return wfapi.FormulaInputSimple{}, nil, err
			}
		}

		return wfapi.FormulaInputSimple{
			WareID: wareId,
		}, wareAddrs, nil

	case basis.Pipe != nil:
		// resolve the pipe to a WareID using the pipeCtx
		input, err := pipeCtx.lookup(basis.Pipe.StepName, basis.Pipe.Label)
		return *input.Basis(), nil, err

	case basis.Ingest != nil && basis.Ingest.GitIngest != nil:
		input := wfapi.FormulaInputSimple{
			WareID: &wfapi.WareID{},
		}

		path, errRaw := filepath.Abs(basis.Ingest.GitIngest.HostPath) //FIXME: This will always be relative to os.Getwd
		if errRaw != nil {
			return wfapi.FormulaInputSimple{}, nil, wfapi.ErrorIo("failed to convert git host path to absolute path", basis.Ingest.GitIngest.HostPath, errRaw)
		}

		// populate cache dir with git ingest
		//
		// note, this executes on the host, not in a container. however, this does work, because it will be checked out
		// and owned by the same user that invokes runc, resulting in all files being owned by uid 0 within the container.
		// this doesn't work for tarballs (which preserve persmissions) but does work for git (which does not).
		//
		// since the cache dir will be populated before formula exec occurs, the rio unpack step will
		// be skipped for this input.
		homeWs, err := workspace.OpenHomeWorkspace(os.DirFS("/")) //FIXME: homeworkspace should be passed in
		if err != nil {
			//FIXME: You probably want to _make_ this workspace if it doesn't exist.
			return input, nil, err
		}

		// resolve the revision of the git ingest to a hash
		gitCtx, gitSpan := tracing.Start(ctx, "clone git repository", trace.WithAttributes(tracing.AttrFullExecNameGit, tracing.AttrFullExecOperationGitClone))
		defer gitSpan.End()
		repo, gitErr := git.CloneContext(gitCtx, memory.NewStorage(), nil, &git.CloneOptions{
			URL: "file://" + path,
		})
		tracing.EndWithStatus(gitSpan, gitErr)
		if gitErr != nil {
			return input, nil, wfapi.ErrorGit(fmt.Sprintf("failed to checkout git repository at %q to memory", path), gitErr)
		}

		hashBytes, gitErr := repo.ResolveRevision(plumbing.Revision(basis.Ingest.GitIngest.Ref))
		if gitErr != nil {
			return input, nil, wfapi.ErrorGit(fmt.Sprintf("failed to resolve git revision for repository %q", path), gitErr)
		}

		// create our formula ware id using the resolved hash
		input.WareID.Hash = hashBytes.String()
		input.WareID.Packtype = "git"

		// checkout the git repository to the cache path
		cachePath, _err := homeWs.CachePath(*input.WareID)
		if _err != nil {
			// Error Codes -= warpforge-error-wareid-invalid
			return input, nil, wfapi.ErrorPlotInvalid(fmt.Sprintf("plot contains invalid WareID %q", *input.WareID))
		}
		if _, errRaw = os.Stat(cachePath); os.IsNotExist(errRaw) {
			gitCtx, gitSpan := tracing.Start(ctx, "checkout git ingest", trace.WithAttributes(tracing.AttrFullExecNameGit, tracing.AttrFullExecOperationGitClone))
			defer gitSpan.End()
			_, gitErr = git.PlainCloneContext(gitCtx, cachePath, false, &git.CloneOptions{
				URL:               "file://" + path,
				RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
			})
			tracing.EndWithStatus(gitSpan, gitErr)

			if gitErr != nil {
				return input, nil, wfapi.ErrorGit(fmt.Sprintf("failed to checkout git ingest for repository %s", path), gitErr)
			}
		}
		return input, nil, nil

	case basis.Literal != nil:
		// pass through the literal value
		return wfapi.FormulaInputSimple{
			Literal: basis.Literal,
		}, nil, nil
	}
	return wfapi.FormulaInputSimple{}, nil, wfapi.ErrorPlotInvalid("invalid type in plot input")
}

func execReplay(ctx context.Context, wss workspace.WorkspaceSet, ref wfapi.CatalogRef, cfg ExecConfig, plotCfg wfapi.PlotExecConfig, expected wfapi.WareID) error {
	ctx, span := tracing.Start(ctx, "execReplay")
	defer span.End()
	span.SetAttributes(attribute.KeyValue{Key: "ref", Value: attribute.StringValue(ref.String())})
	logger := logging.Ctx(ctx)

	if !plotCfg.Recursive {
		// recursion is not allowed, return
		//REVIEW: Do we need to signal lack of execution
		return nil
	}
	replay, err := wss.GetCatalogReplay(ref)
	if err != nil {
		return err
	}
	if replay == nil {
		// REVIEW: Probably need an error to signal no replay executed
		return nil
	}
	logger.Info(LOG_TAG, "resolving replay for module = %s, release = %s...",
		ref.ModuleName, ref.ReleaseName)
	result, err := execPlot(ctx, cfg, wss, *replay, plotCfg)
	if err != nil {
		return wfapi.ErrorPlotStepFailed("replay", err)
	}
	replayWareId, hasItem := result.Values[wfapi.LocalLabel(ref.ItemName)]
	if !hasItem {
		return wfapi.ErrorPlotInvalid(
			fmt.Sprintf("replay doesn't have item %q", wfapi.LocalLabel(ref.ItemName)))
	}
	if replayWareId != expected {
		return wfapi.ErrorPlotInvalid(
			fmt.Sprintf("replay failed to produce correct WareID for item %q. expected %q, replay WareID is %q",
				ref.ItemName, expected, replayWareId))
	}
	return nil
}

// Executes a protoformula within a Plot
//
// Errors:
//
//    - warpforge-error-io -- when an IO error occurs
//    - warpforge-error-formula-execution-failed -- when an error occurs during formula execution
//    - warpforge-error-executor-failed -- when the execution step of the formula fails
//    - warpforge-error-ware-unpack -- when a ware unpack operation fails for a formula input
//    - warpforge-error-ware-pack -- when a ware pack operation fails for a formula output
//    - warpforge-error-formula-invalid -- when an invalid formula is provided
//    - warpforge-error-git -- when an error handing a git ingest occurs
//    - warpforge-error-catalog-parse -- when parsing of catalog files fails
//    - warpforge-error-catalog-missing-entry -- when a referenced catalog entry cannot be found
//    - warpforge-error-plot-invalid -- when the plot contains invalid data
//    - warpforge-error-catalog-invalid -- when the catalog contains invalid data
//    - warpforge-error-plot-step-failed -- when a replay fails
//    - warpforge-error-serialization -- when serialization or deserialization of a memo fails
//    - warpforge-error-workspace-missing -- when home workspace is missing or cannot open
func execProtoformula(ctx context.Context,
	cfg ExecConfig,
	wss workspace.WorkspaceSet,
	pf wfapi.Protoformula,
	formulaCtx wfapi.FormulaContext,
	plotCfg wfapi.PlotExecConfig,
	pipeCtx pipeMap) (wfapi.RunRecord, error) {
	ctx, span := tracing.Start(ctx, "execProtoformula")
	defer span.End()
	logger := logging.Ctx(ctx)

	// create an empty Formula and FormulaContext
	formula := wfapi.Formula{
		Action: pf.Action,
	}
	formula.Inputs.Values = make(map[wfapi.SandboxPort]wfapi.FormulaInput)
	formula.Outputs.Values = make(map[wfapi.OutputName]wfapi.GatherDirective)

	// convert Protoformula inputs (of type PlotInput) to FormulaInputs
	for sbPort, plotInput := range pf.Inputs.Values {
		formula.Inputs.Keys = append(formula.Inputs.Keys, sbPort)
		input, wareAddrs, err := plotInputToFormulaInput(ctx, cfg, wss, plotInput, plotCfg, pipeCtx)
		if err != nil {
			return wfapi.RunRecord{}, err
		}
		formula.Inputs.Values[sbPort] = input
		if len(wareAddrs) != 0 {
			// input specifies a WarehouseAddr, add it to the formula's context
			formulaCtx.Warehouses.Keys = append(formulaCtx.Warehouses.Keys, *input.Basis().WareID)
			//TODO: Change WF API for multiple warehouses per ware ID
			logger.Info(LOG_TAG, "\t\t%s = %s",
				color.HiBlueString("warehouse"),
				color.WhiteString("%s", wareAddrs[0]),
			)
			formulaCtx.Warehouses.Values[*input.Basis().WareID] = wareAddrs[0]
		}
	}

	// convert Protoformula outputs to Formula outputs
	for label, gatherDirective := range pf.Outputs.Values {
		label := wfapi.OutputName(label)
		formula.Outputs.Keys = append(formula.Outputs.Keys, label)
		formula.Outputs.Values[label] = gatherDirective
	}

	// execute the derived formula
	rr, err := formulaexec.Exec(ctx, formulaexec.ExecConfig(cfg), wss.Root(),
		wfapi.FormulaAndContext{
			Formula: wfapi.FormulaCapsule{Formula: &formula},
			Context: &wfapi.FormulaContextCapsule{FormulaContext: &formulaCtx},
		}, plotCfg.FormulaExecConfig)
	return rr, err
}

// Execute a Plot using the provided WorkspaceSet
// This is an internal function which takes a V1 plot and is called recursively
//
// Errors:
//
//    - warpforge-error-plot-invalid -- when the provided plot input is invalid
//    - warpforge-error-catalog-missing-entry -- when a referenced catalog reference cannot be found
//    - warpforge-error-git -- when a git related error occurs during a git ingest
//    - warpforge-error-io -- when an IO error occurs during conversion
//    - warpforge-error-catalog-parse -- when parsing of catalog files fails
//    - warpforge-error-catalog-invalid -- when the catalog contains invalid data
//    - warpforge-error-plot-step-failed -- when execution of a plot step fails
//    - warpforge-error-workspace-missing -- when home workspace is missing or cannot be opened
func execPlot(ctx context.Context, cfg ExecConfig, wss workspace.WorkspaceSet, plot wfapi.Plot, pltCfg wfapi.PlotExecConfig) (wfapi.PlotResults, error) {
	ctx, span := tracing.Start(ctx, "execPlot")
	defer span.End()
	pipeCtx := make(pipeMap)
	results := wfapi.PlotResults{}
	logger := logging.Ctx(ctx)
	logger.Info(LOG_TAG_START, "")
	defer logger.Info(LOG_TAG_END, "")
	cfg.debug(ctx)
	for idx, ws := range wss {
		logger.Debug(LOG_TAG, "workspace[%d]: %s", idx, ws.InternalPath())
	}

	// collect the plot inputs
	// these have an empty string for the step name (e.g., `pipe::foo`)
	logger.Info(LOG_TAG, "inputs:")
	pipeCtx[""] = make(map[wfapi.LocalLabel]wfapi.FormulaInput)
	inputContext := wfapi.FormulaContext{}
	inputContext.Warehouses.Values = make(map[wfapi.WareID]wfapi.WarehouseAddr)
	for name, input := range plot.Inputs.Values {
		input, wareAddrs, err := plotInputToFormulaInput(ctx, cfg, wss, input, pltCfg, pipeCtx)
		if err != nil {
			return results, err
		}
		pipeCtx[""][name] = input
		if len(wareAddrs) != 0 {
			// input specifies an address, add it to the context
			inputContext.Warehouses.Keys = append(inputContext.Warehouses.Keys, *input.Basis().WareID)
			inputContext.Warehouses.Values[*input.Basis().WareID] = wareAddrs[0] // TODO: handle multiple warehouses
		}
	}

	// determine step execution order
	stepsOrdered, err := OrderSteps(ctx, plot)
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
			rr, err := execProtoformula(ctx, cfg, wss, *step.Protoformula, inputContext, pltCfg, pipeCtx)
			if err != nil {
				return results, wfapi.ErrorPlotStepFailed(name, err)
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
					FormulaInputSimple: &wfapi.FormulaInputSimple{
						WareID:  input.WareID,
						Literal: input.Literal,
						Mount:   input.Mount,
					},
				}
			}
		case step.Plot != nil:
			// execute plot step
			logger.Info(LOG_TAG_MID, "(%s) %s",
				color.HiCyanString(string(name)),
				color.WhiteString("evaluating subplot"),
			)

			stepResults, err := execPlot(ctx, cfg, wss, *step.Plot, pltCfg)
			if err != nil {
				return results, wfapi.ErrorPlotStepFailed(name, err)
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
			return results, wfapi.ErrorPlotInvalid(fmt.Sprintf("plot step %q does not contain a Protoformula or Plot", name))
		}

		logger.Info(LOG_TAG_MID, "(%s) %s",
			color.HiCyanString(string(name)),
			color.WhiteString("complete"),
		)
		logger.Info(LOG_TAG, "")
	}

	// collect the outputs of this plot
	results.Values = make(map[wfapi.LocalLabel]wfapi.WareID)
	for name, output := range plot.Outputs.Values {
		result, err := pipeCtx.lookup(output.Pipe.StepName, output.Pipe.Label)
		if err != nil {
			return results, err
		}
		results.Keys = append(results.Keys, name)
		results.Values[name] = *result.Basis().WareID
	}

	// TODO: This is currently the primary output mechanism of warpforge.
	// This makes controlling UX a lot harder than it should be.
	logger.PrintPlotResults(LOG_TAG, results)

	return results, nil
}

// Execute a PlotCapsule using the provided WorkspaceSet
//
// Errors:
//
//    - warpforge-error-catalog-invalid -- when the catalog contains invalid data
//    - warpforge-error-catalog-missing-entry -- when a referenced catalog reference cannot be found
//    - warpforge-error-catalog-parse -- when parsing of catalog files fails
//    - warpforge-error-git -- when a git related error occurs during a git ingest
//    - warpforge-error-io -- when an IO error occurs during conversion
//    - warpforge-error-plot-invalid -- when the provided plot input is invalid
//    - warpforge-error-plot-step-failed -- when execution of a plot step fails
//    - warpforge-error-workspace-missing -- when home workspace is missing or cannot be opened
func Exec(ctx context.Context, cfg ExecConfig, wss workspace.WorkspaceSet, plotCapsule wfapi.PlotCapsule, pltCfg wfapi.PlotExecConfig) (result wfapi.PlotResults, err error) {
	ctx, span := tracing.StartFn(ctx, "Exec")
	defer func() { tracing.EndWithStatus(span, err) }()
	if plotCapsule.Plot == nil {
		return wfapi.PlotResults{}, wfapi.ErrorPlotInvalid("PlotCapsule does not contain a v1 plot")
	}
	return execPlot(ctx, cfg, wss, *plotCapsule.Plot, pltCfg)
}
