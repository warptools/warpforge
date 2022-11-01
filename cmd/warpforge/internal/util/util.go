package util

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"

	"github.com/warpfork/warpforge/pkg/plotexec"
	"github.com/warpfork/warpforge/pkg/tracing"
	"github.com/warpfork/warpforge/pkg/workspace"
	"github.com/warpfork/warpforge/wfapi"
)

// special file names for plot and module files
// these are json files with special formatting for detection
const (
	PlotFilename   = "plot.wf"
	ModuleFilename = "module.wf"
)

// GetFileType returns the file type, which is the file name without extension
// e.g., formula.wf -> formula, module.wf -> module, etc...
//
// Errors:
//
//   - warpforge-error-invalid -- when extension is unsupported
func GetFileType(name string) (string, error) {
	split := strings.Split(filepath.Base(name), ".")
	if len(split) != 2 {
		// ignore files without extensions
		//TODO: pick a better error code
		return "", wfapi.ErrorInvalid(fmt.Sprintf("unsupported file: %q", name), [2]string{"name", name})
	}
	return split[0], nil
}

// BinPath is a helper function for finding the path to internally used binaries (e.g, rio, runc)
//
// Errors:
//
//    - warpforge-error-unknown -- When the path to this executable can't be found
func BinPath(bin string) (string, error) {
	path, override := os.LookupEnv("WARPFORGE_PATH")
	if override {
		return filepath.Join(path, bin), nil
	}

	path, err := os.Executable()
	if err != nil {
		return "", wfapi.ErrorUnknown("unable to get path of warpforge executable", err)
	}

	return filepath.Join(filepath.Dir(path), bin), nil
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
func OpenWorkspaceSet() (workspace.WorkspaceSet, wfapi.Error) {
	pwd, err := os.Getwd()
	if err != nil {
		return workspace.WorkspaceSet{}, wfapi.ErrorUnknown("failed to get working directory: %s", err)
	}

	wss, err := workspace.FindWorkspaceStack(os.DirFS("/"), "", pwd[1:])
	if err != nil {
		return workspace.WorkspaceSet{}, wfapi.ErrorWorkspace(pwd, err)
	}
	return wss, nil
}

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

// ModuleFromFile takes a path to a module file, returns a module
// Errors:
//
//     - warpforge-error-io -- when the file cannot be read
//     - warpforge-error-module-invalid -- when the module data is invalid
//     - warpforge-error-serialization -- when the module doesn't parse
func ModuleFromFile(filename string) (wfapi.Module, error) {
	f, err := ioutil.ReadFile(filename)
	if err != nil {
		return wfapi.Module{}, wfapi.ErrorIo("unable to read module", filename, err)
	}

	moduleCapsule := wfapi.ModuleCapsule{}
	_, err = ipld.Unmarshal(f, json.Decode, &moduleCapsule, wfapi.TypeSystem.TypeByName("ModuleCapsule"))
	if err != nil {
		return wfapi.Module{}, wfapi.ErrorSerialization("unable to deserialize module", err)
	}
	if moduleCapsule.Module == nil {
		return wfapi.Module{}, wfapi.ErrorModuleInvalid("no v1 Module in ModuleCapsule")
	}

	return *moduleCapsule.Module, nil
}

// ExecModule executes the given module file with the default plot file in the same directory.
// Errors:
//
//    - warpforge-error-catalog-invalid --
//    - warpforge-error-catalog-parse --
//    - warpforge-error-git --
//    - warpforge-error-io -- when the module or plot files cannot be read or cannot change directory.
//    - warpforge-error-missing-catalog-entry --
//    - warpforge-error-module-invalid -- when the module data is invalid
//    - warpforge-error-plot-execution-failed --
//    - warpforge-error-plot-invalid -- when the plot data is invalid
//    - warpforge-error-plot-step-failed --
//    - warpforge-error-serialization -- when the module or plot cannot be parsed
//    - warpforge-error-unknown -- when changing directories fails
//    - warpforge-error-workspace -- when opening the workspace set fails
func ExecModule(ctx context.Context, config wfapi.PlotExecConfig, fileName string) (wfapi.PlotResults, error) {
	ctx, span := tracing.Start(ctx, "execModule")
	defer span.End()
	result := wfapi.PlotResults{}

	// parse the module, even though it is not currently used
	if _, werr := ModuleFromFile(fileName); werr != nil {
		return result, werr
	}

	plot, werr := PlotFromFile(filepath.Join(filepath.Dir(fileName), PlotFilename))
	if werr != nil {
		return result, werr
	}

	pwd, nerr := os.Getwd()
	if nerr != nil {
		return result, wfapi.ErrorUnknown("unable to get pwd", nerr)
	}

	wss, werr := OpenWorkspaceSet()
	if werr != nil {
		return result, wfapi.ErrorWorkspace(pwd, werr)
	}

	tmpDir := filepath.Dir(fileName)
	// FIXME: it would be nice if we could avoid changing directories.
	//  This generally means removing Getwd calls from pkg libs
	if nerr := os.Chdir(tmpDir); nerr != nil {
		return result, wfapi.ErrorIo("cannot change directory", tmpDir, nerr)
	}

	result, werr = plotexec.Exec(ctx, wss, wfapi.PlotCapsule{Plot: &plot}, config)

	if nerr := os.Chdir(pwd); nerr != nil {
		return result, wfapi.ErrorIo("cannot return to pwd", pwd, nerr)
	}

	if werr != nil {
		return result, wfapi.ErrorPlotExecutionFailed(werr)
	}

	return result, nil
}