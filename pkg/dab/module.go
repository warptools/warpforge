package dab

import (
	"fmt"
	"io/fs"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"

	"github.com/warpfork/warpforge/wfapi"
)

const (
	MagicFilename_Module = "module.wf"
	MagicFilename_Plot   = "plot.wf"
)

// ModuleFromFile loads a wfapi.Module from filesystem path.
//
// In typical usage, the filename parameter will have the suffix of MagicFilename_Module.
//
// Errors:
//
// 	- warpforge-error-io -- for errors reading from fsys.
// 	- warpforge-error-serialization -- for errors from try to parse the data as a Module.
// 	- warpforge-error-datatoonew -- if encountering unknown data from a newer version of warpforge!
//
func ModuleFromFile(fsys fs.FS, filename string) (wfapi.Module, error) {
	situation := "loading a module"

	f, err := fs.ReadFile(fsys, filename)
	if err != nil {
		return wfapi.Module{}, wfapi.ErrorIo(situation, &filename, err)
	}

	moduleCapsule := wfapi.ModuleCapsule{}
	_, err = ipld.Unmarshal(f, json.Decode, &moduleCapsule, wfapi.TypeSystem.TypeByName("ModuleCapsule"))
	if err != nil {
		return wfapi.Module{}, wfapi.ErrorSerialization(situation, err)
	}
	if moduleCapsule.Module == nil {
		// ... this isn't really reachable.
		return wfapi.Module{}, wfapi.ErrorDataTooNew(situation, fmt.Errorf("no v1 Module in ModuleCapsule"))
	}

	return *moduleCapsule.Module, nil
}

// PlotFromFile loads a wfapi.Plot from filesystem path.
//
// In typical usage, the filename parameter will have the suffix of MagicFilename_Plot.
//
// Errors:
//
// 	- warpforge-error-io -- for errors reading from fsys.
// 	- warpforge-error-serialization -- for errors from try to parse the data as a Plot.
// 	- warpforge-error-datatoonew -- if encountering unknown data from a newer version of warpforge!
//
func PlotFromFile(fsys fs.FS, filename string) (wfapi.Plot, error) {
	situation := "loading a plot"

	f, err := fs.ReadFile(fsys, filename)
	if err != nil {
		return wfapi.Plot{}, wfapi.ErrorIo(situation, &filename, err)
	}

	plotCapsule := wfapi.PlotCapsule{}
	_, err = ipld.Unmarshal(f, json.Decode, &plotCapsule, wfapi.TypeSystem.TypeByName("PlotCapsule"))
	if err != nil {
		return wfapi.Plot{}, wfapi.ErrorSerialization(situation, err)
	}
	if plotCapsule.Plot == nil {
		// ... this isn't really reachable.
		return wfapi.Plot{}, wfapi.ErrorDataTooNew(situation, fmt.Errorf("no v1 Plot in PlotCapsule"))
	}

	return *plotCapsule.Plot, nil
}
