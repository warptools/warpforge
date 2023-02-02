package util

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/serum-errors/go-serum"
	"go.opentelemetry.io/otel/codes"

	"github.com/warptools/warpforge/pkg/config"
	"github.com/warptools/warpforge/pkg/dab"
	"github.com/warptools/warpforge/pkg/plotexec"
	"github.com/warptools/warpforge/pkg/tracing"
	"github.com/warptools/warpforge/pkg/workspace"
	"github.com/warptools/warpforge/wfapi"
)

// OpenWorkspaceSet opens the default WorkspaceSet.
// This consists of:
//   workspace stack: a workspace stack starting at the current working directory,
//    root workspace: the first marked root workspace in the stack, or the home workspace if none are marked,
//    home workspace: the workspace at the user's homedir
//
// Errors:
//
//    - warpforge-error-workspace -- could not load workspace stack
//    - warpforge-error-unknown -- failed to get working directory
func OpenWorkspaceSet() (workspace.WorkspaceSet, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return workspace.WorkspaceSet{}, serum.Errorf(wfapi.ECodeUnknown, "failed to get working directory: %w", err)
	}

	wss, err := workspace.FindWorkspaceStack(os.DirFS("/"), "", pwd[1:])
	if err != nil {
		return workspace.WorkspaceSet{}, wfapi.ErrorWorkspace(pwd, err)
	}
	return wss, nil
}

// DEPRECATED: use dab package
// PlotFromFile takes a path to a plot file, returns a plot
// Errors:
//
//    - warpforge-error-io -- plot file cannot be read
//    - warpforge-error-plot-invalid -- plot data is invalid
//    - warpforge-error-serialization -- plot file cannot be serialized into a plot
func PlotFromFile(filename string) (wfapi.Plot, error) {
	f, err := ioutil.ReadFile(filename)
	if err != nil {
		return wfapi.Plot{}, wfapi.ErrorIo("unable to read plot file", filename, err)
	}

	plotCapsule := wfapi.PlotCapsule{}
	_, err = ipld.Unmarshal(f, json.Decode, &plotCapsule, wfapi.TypeSystem.TypeByName("PlotCapsule"))
	if err != nil {
		return wfapi.Plot{}, wfapi.ErrorSerialization("unable to deserialize plot", err)
	}
	if plotCapsule.Plot == nil {
		return wfapi.Plot{}, wfapi.ErrorPlotInvalid("no v1 Plot in PlotCapsule")
	}

	return *plotCapsule.Plot, nil
}

// canonicalize is like filepath.Abs but assumes we already have a working directory path which is absolute
func canonicalizePath(pwd, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	if !filepath.IsAbs(pwd) {
		panic(fmt.Sprintf("working directory must be an absolute path: %q", pwd))
	}
	return filepath.Join(pwd, path)
}

// ExecModule executes the given module file with the default plot file in the same directory.
//
// Errors:
//
//    - warpforge-error-catalog-invalid --
//    - warpforge-error-catalog-parse --
//    - warpforge-error-git --
//    - warpforge-error-missing -- when a required file is missing
//    - warpforge-error-io -- when the module or plot files cannot be read or cannot change directory.
//    - warpforge-error-catalog-missing-entry --
//    - warpforge-error-module-invalid -- when the module data is invalid
//    - warpforge-error-plot-execution-failed --
//    - warpforge-error-plot-invalid -- when the plot data is invalid
//    - warpforge-error-plot-step-failed --
//    - warpforge-error-serialization -- when the module or plot cannot be parsed
//    - warpforge-error-workspace-missing -- when opening the workspace set fails
//    - warpforge-error-datatoonew -- when error is too new
//    - warpforge-error-searching-filesystem -- unexpected error traversing filesystem
//    - warpforge-error-initialization -- fail to get working directory or executable path
func ExecModule(ctx context.Context, wss workspace.WorkspaceSet, pltCfg wfapi.PlotExecConfig, fileName string) (result wfapi.PlotResults, err error) {
	ctx, span := tracing.StartFn(ctx, "execModule")
	defer func() { tracing.EndWithStatus(span, err) }()

	fsys := os.DirFS("/")

	// parse the module, even though it is not currently used
	if _, err := dab.ModuleFromFile(fsys, fileName); err != nil {
		return result, err
	}

	moduleDir := filepath.Dir(fileName)
	execCfg, err := config.PlotExecConfig(&moduleDir)
	if err != nil {
		return result, err
	}
	modulePath := canonicalizePath(execCfg.WorkingDirectory, filepath.Dir(fileName))

	if wss == nil {
		var werr error
		wss, werr = workspace.FindWorkspaceStack(fsys, "", modulePath[1:])
		if werr != nil {
			return result, werr
		}
	}

	plot, werr := dab.PlotFromFile(fsys, filepath.Join(modulePath, dab.MagicFilename_Plot))
	if werr != nil {
		return result, werr
	}

	result, werr = plotexec.Exec(ctx, execCfg, wss, wfapi.PlotCapsule{Plot: plot}, pltCfg)

	if werr != nil {
		return result, wfapi.ErrorPlotExecutionFailed(werr)
	}

	span.SetStatus(codes.Ok, "")
	return result, nil
}
