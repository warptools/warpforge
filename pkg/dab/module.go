package dab

import (
	"fmt"
	"io/fs"
	"regexp"
	"strconv"
	"strings"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/serum-errors/go-serum"

	"github.com/warptools/warpforge/wfapi"
)

const (
	MagicFilename_Module = "module.wf"
	MagicFilename_Plot   = "plot.wf"
)

var (
	alphaNumReFmt               = `[a-zA-Z0-9]`
	wordReFmt                   = `[-a-zA-Z0-9_\.]`
	segmentReFmt                = fmt.Sprintf(`(%s%s*)?%s`, alphaNumReFmt, wordReFmt, alphaNumReFmt)
	firstSegmentReFmt           = fmt.Sprintf(`%[1]s\.%[1]s`, segmentReFmt)
	reModuleFirstSegment        = regexp.MustCompile(`^` + firstSegmentReFmt + `$`)
	reModuleName                = regexp.MustCompile(`^` + firstSegmentReFmt + `(/` + segmentReFmt + `)*$`)
	moduleFirstSegmentMaxLength = 63 // limit first segment to encourage compatibility with DNS domain name rules
)

// ValidateModuleName checks the module name for invalid strings.
// Name "path segments" are defined as the segments separated by forward slash "/".
//
// Module names have the following rules:
//    - Name MUST start AND end each segment with an ASCII alpha-numeric character.
//    - Name MUST contain only ASCII alpha-numeric characters plus underscores '_', hyphens '-', dots '.', and forward slash '/'.
//    - First segment of the name MUST include a dot '.' character and must be 63 characters or less
//
// Errors:
//
//  - warpforge-error-module-invalid -- when module name is invalid
func ValidateModuleName(moduleName wfapi.ModuleName) error {
	name := string(moduleName)
	parts := strings.Split(name, "/")
	if !reModuleFirstSegment.MatchString(parts[0]) {
		// test first segment separately so we can add a more specific error message
		return serum.Error(wfapi.ECodeModuleInvalid,
			serum.WithMessageLiteral("first segment of module name must both start and end with an alphanumeric character, must contain at least one '.', and must consist of alphanumeric characters, '-', '_', or '.'"),
			serum.WithDetail("name", strconv.Quote(name)),
		)
	}
	if !reModuleName.MatchString(name) {
		return serum.Error(wfapi.ECodeModuleInvalid,
			serum.WithMessageLiteral("module name segments must both start and end with an alphanumeric character and must consist of alphanumeric characters, '-', '_', or '.'"),
			serum.WithDetail("name", strconv.Quote(name)),
		)
	}
	if len(parts[0]) > moduleFirstSegmentMaxLength {
		return serum.Errorf(wfapi.ECodeModuleInvalid, "first segment of module name may not be longer than %d characters", moduleFirstSegmentMaxLength)
	}
	return nil
}

// ModuleFromFile loads a wfapi.Module from filesystem path.
//
// In typical usage, the filename parameter will have the suffix of MagicFilename_Module.
//
// Errors:
//
// 	- warpforge-error-io -- for errors reading from fsys.
// 	- warpforge-error-serialization -- for errors from try to parse the data as a Module.
// 	- warpforge-error-datatoonew -- if encountering unknown data from a newer version of warpforge!
//  - warpforge-error-module-invalid -- when module name is invalid
func ModuleFromFile(fsys fs.FS, filename string) (wfapi.Module, error) {
	const situation = "loading a module"
	if strings.HasPrefix(filename, "/") {
		filename = filename[1:]
	}
	f, err := fs.ReadFile(fsys, filename)
	if err != nil {
		return wfapi.Module{}, wfapi.ErrorIo(situation, filename, err)
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

	if err := ValidateModuleName(moduleCapsule.Module.Name); err != nil {
		return wfapi.Module{}, err
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
func PlotFromFile(fsys fs.FS, filename string) (wfapi.Plot, error) {
	const situation = "loading a plot"

	if strings.HasPrefix(filename, "/") {
		filename = filename[1:]
	}
	f, err := fs.ReadFile(fsys, filename)
	if err != nil {
		return wfapi.Plot{}, wfapi.ErrorIo(situation, filename, err)
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
