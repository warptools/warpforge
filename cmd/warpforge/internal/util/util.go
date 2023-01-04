package util

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/serum-errors/go-serum"

	"github.com/warptools/warpforge/pkg/dab"
	"github.com/warptools/warpforge/pkg/formulaexec"
	"github.com/warptools/warpforge/pkg/plotexec"
	"github.com/warptools/warpforge/pkg/tracing"
	"github.com/warptools/warpforge/pkg/workspace"
	"github.com/warptools/warpforge/wfapi"
)

// BinPath is a helper function for finding the path to internally used binaries (e.g, rio, runc)
//
// Errors:
//
//    - warpforge-error-io -- When the path to this executable can't be found
func BinPath(bin string) (string, error) {
	path, err := formulaexec.GetBinPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(path, bin), nil
}

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

// ExecModule executes the given module file with the default plot file in the same directory.
// WARNING: This function calls Chdir and may not change back on errors
//
// Errors:
//
//    - warpforge-error-catalog-invalid --
//    - warpforge-error-catalog-parse --
//    - warpforge-error-git --
//    - warpforge-error-io -- when the module or plot files cannot be read or cannot change directory.
//    - warpforge-error-catalog-missing-entry --
//    - warpforge-error-module-invalid -- when the module data is invalid
//    - warpforge-error-plot-execution-failed --
//    - warpforge-error-plot-invalid -- when the plot data is invalid
//    - warpforge-error-plot-step-failed --
//    - warpforge-error-serialization -- when the module or plot cannot be parsed
//    - warpforge-error-unknown -- when changing directories fails
//    - warpforge-error-workspace -- when opening the workspace set fails
//    - warpforge-error-datatoonew -- when module or plot data version is unrecognized
func ExecModule(ctx context.Context, config wfapi.PlotExecConfig, fileName string) (wfapi.PlotResults, error) {
	ctx, span := tracing.Start(ctx, "execModule")
	defer span.End()
	result := wfapi.PlotResults{}

	fsys := os.DirFS("/")
	// parse the module, even though it is not currently used
	if _, err := dab.ModuleFromFile(fsys, fileName); err != nil {
		return result, err
	}

	plot, werr := dab.PlotFromFile(fsys, filepath.Join(filepath.Dir(fileName), dab.MagicFilename_Plot))
	if werr != nil {
		return result, werr
	}

	pwd, err := os.Getwd()
	if err != nil {
		return result, serum.Errorf(wfapi.ECodeUnknown, "unable to get pwd: %w", err)
	}

	wss, err := OpenWorkspaceSet()
	if err != nil {
		return result, wfapi.ErrorWorkspace(pwd, err)
	}

	tmpDir := filepath.Dir(fileName)
	// FIXME: it would be nice if we could avoid changing directories.
	//  This generally means removing Getwd calls from pkg libs
	if err := os.Chdir(tmpDir); err != nil {
		return result, wfapi.ErrorIo("cannot change directory", tmpDir, err)
	}

	result, werr = plotexec.Exec(ctx, wss, wfapi.PlotCapsule{Plot: &plot}, config)

	if err := os.Chdir(pwd); err != nil {
		return result, wfapi.ErrorIo("cannot return to pwd", pwd, err)
	}

	if werr != nil {
		return result, wfapi.ErrorPlotExecutionFailed(werr)
	}

	return result, nil
}
