package dab

import (
	"bufio"
	"errors"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/serum-errors/go-serum"
	"github.com/warptools/warpforge/wfapi"
)

// ActionableSearch is a bitfield for what the ActionableFromFS function should treat as acceptable findings.
type ActionableSearch uint8

const (
	ActionableSearch_None    ActionableSearch = 0
	ActionableSearch_Formula ActionableSearch = 1      // Use this bitflag to indicate a formula is an acceptable search result.
	ActionableSearch_Module  ActionableSearch = 1 << 1 // Use this bitflag to indicate a module is an acceptable search result.  Modules almost always have a plot associated with them, which will also be loaded by most search functions.
	ActionableSearch_Plot    ActionableSearch = 1 << 2 // A module can always also have a plot; use this if a bare plot (no module) is acceptable.

	ActionableSearch_Any ActionableSearch = ActionableSearch_Formula | ActionableSearch_Module | ActionableSearch_Plot
)

func stat(fsys fs.FS, path string) (fs.FileInfo, error) {
	if filepath.IsAbs(path) {
		path = path[1:]
	}
	fi, err := fs.Stat(fsys, path)
	if err != nil {
		return nil, serum.Error(wfapi.ECodeMissing, serum.WithCause(err))
	}
	return fi, nil
}

func open(fsys fs.FS, path string) (fs.File, error) {
	if filepath.IsAbs(path) {
		path = path[1:]
	}
	f, err := fsys.Open(path)
	if err != nil {
		return f, serum.Error(wfapi.ECodeIo, serum.WithCause(err))
	}
	return f, nil
}

// FindActionableFromFS loads either module (and plot) from the fileystem,
// or instead a Formula,
// while also accepting directories as input and applying reasonable heuristics.
//
// FindActionableFromFS is suitable for finding *one* module/plot/formula;
// finding groupings of modules (i.e., handling args of "./..." forms) is a different feature.
//
// The 'fsys' parameter is typically `os.DirFS("/")` except in test environments.
//
// The 'basisPath' and 'searchPath' parameters work together to specify which paths to inspect.
// The `{basisPath}/{searchPath}` path will always be searched;
// and if the 'searchUp' parameter is true, then each segment of 'searchPath'
// will also be popped and searched.
// (If 'searchUp' is false, the distinction between 'basisPath' and 'searchPath' vanishes.)
//
// If `{basisPath}/{searchPath}` contains a file, only file is inspected,
// and search up through parent directories is not performed regardless of the value of 'searchUp'.
//
// The typical values of 'basisPath' and 'searchPath' vary depending on feature, but are usually one of:
// basisPath is the CWD, and searchPath was a CLI argument;
// or, basisPath is empty string (meaning the root of the filesystem), and searchPath is the CWD (possibly joined with a CLI argument);
// or, basisPath is a workspace directory, and searchPath is the CWD within that (possibly joined with a CLI argument).
// Neither value should ever have a leading slash (as is typical for APIs using FS).
//
// The 'accept' parameter can be used to have the function ignore (or return errors) in some scenarios.
// For example, 'ActionableSearch_Any' may cause the search to return a formula or a module;
// whereas if you only want only want to detect modules, you should use 'ActionableSearch_Module' instead.
// See the comments on the constants for details on other options.
//
// It is possible for a module and plot to be returned;
// or a module alone; or a plot alone; or a formula alone;
// or an error; or all four of them may be nil at once if the search found nothing.
// The foundPath and remainingSearchPath values are always returned,
// even in the case of errors.
//
// Errors:
//
//  - warpforge-error-datatoonew -- when found data is not supported by this version of warpforge
//  - warpforge-error-invalid-argument -- when a file contains the wrong content for its filename
//  - warpforge-error-io -- when an IO error occurs during search
//  - warpforge-error-missing -- when an expected file is missing
//  - warpforge-error-module-invalid -- when a read module contains invalid data
//  - warpforge-error-searching-filesystem -- when the search of the filesystem produces an invalid result
//  - warpforge-error-serialization -- when IPLD deserialization fails
func FindActionableFromFS(
	fsys fs.FS,
	basisPath string, searchPath string, searchUp bool,
	accept ActionableSearch,
) (
	m *wfapi.Module, p *wfapi.Plot, f *wfapi.Formula,
	foundPath string, remainingSearchPath string, err error,
) {
	remainingSearchPath = searchPath

	// First round: this may be a file, and may be one of any of the types.
	// Stat it and consider that first:
	// if it is a file, that's its own whole detection procedure (and means no further search);
	// if it's not a file, we can procede to the behavior for dir feature detection (without popping).
	foundPath = filepath.Join(basisPath, remainingSearchPath)
	fi, e2 := stat(fsys, foundPath)
	if e2 != nil {
		err = e2
		return
	}
	if fi.Mode()&^fs.ModePerm == 0 {
		fh, e2 := open(fsys, foundPath)
		defer fh.Close()
		if e2 != nil {
			err = e2
			return
		}
		m, p, f, err = findActionableFromFile(fh, accept)
		if m != nil { // If a module was found, a plot may also exist as a sibling file.
			p, e2 = PlotFromFile(fsys, filepath.Join(filepath.Dir(filepath.Join(basisPath, remainingSearchPath)), "plot.wf"))
			if !errors.Is(e2, fs.ErrNotExist) {
				err = wfapi.ErrorSearchingFilesystem("loading plot associated with a module", e2)
			}
		}
		return
	}

	// Iteratively check directories.
	// In each directory, look for well-known file names.
	// If we find a module, that dominates; then plot; then formula.
	// When there's no match, if searchUp is enabled, then pop one segment off searchPath and search again.
	for {
		// Peek for module.
		if accept&ActionableSearch_Module > 0 {
			foundPath = filepath.Join(basisPath, remainingSearchPath, "module.wf")
			m, e2 = ModuleFromFile(fsys, foundPath)
			if serum.Code(e2) != wfapi.ECodeMissing { // notexist is ignored.
				// Any error that's just just notexist: means our search has blind spots: error out.
				err = wfapi.ErrorSearchingFilesystem("modules, plots, or formulas", e2)
				return
			}
			// A module may also have a plot next to it; load that eagerly too.
			p, e2 = PlotFromFile(fsys, filepath.Join(basisPath, remainingSearchPath, "plot.wf"))
			if serum.Code(e2) != wfapi.ECodeMissing {
				err = wfapi.ErrorSearchingFilesystem("loading plot associated with a module", e2)
			}
			return
		}
		// Peek for plot.
		if accept&ActionableSearch_Plot > 0 {
			foundPath = filepath.Join(basisPath, remainingSearchPath, "plot.wf")
			p, e2 = PlotFromFile(fsys, foundPath)
			if serum.Code(e2) != wfapi.ECodeMissing { // notexist is ignored.
				// Any error that's just just notexist: means our search has blind spots: error out.
				err = wfapi.ErrorSearchingFilesystem("modules, plots, or formulas", e2)
				return
			}
		}

		// Peek for Formula.
		// TODO

		// None of the filename peeks found a thing?
		// Okay.  If we can search further up, let's do so.
		if !searchUp {
			foundPath = ""
			return
		}
		remainingSearchPath = filepath.Dir(remainingSearchPath)
		if remainingSearchPath == "/" || remainingSearchPath == "." {
			return
		}
	}
}

// findActionableFromFile peeks at the first few bytes of a stream
// to guess whether it's a module, a plot, or a formula, and returns the right one.
func findActionableFromFile(r io.Reader, accept ActionableSearch) (
	m *wfapi.Module, p *wfapi.Plot, f *wfapi.Formula, err error,
) {
	peekSize := 1024
	br := bufio.NewReaderSize(r, peekSize)
	peek, _ := br.Peek(peekSize)
	marker, err := GuessDocumentType(peek, []string{
		"module.v1",
		"plot.v1",
		"formula",
	})
	if err != nil {
		return
	}
	switch marker {
	case "module.v1":
		if accept&ActionableSearch_Module == 0 {
			err = serum.Error(wfapi.ECodeArgument,
				serum.WithMessageLiteral("file contained a module document when a module is not expected"),
			)
			return
		}
		capsule := wfapi.ModuleCapsule{}
		_, e2 := ipld.UnmarshalStreaming(br, json.Decode, &capsule, wfapi.TypeSystem.TypeByName("ModuleCapsule"))
		if e2 != nil {
			err = wfapi.ErrorSerialization("parsing what appeared to be a module", err)
		} else {
			m = capsule.Module
		}
		return
	case "plot.v1":
		if accept&ActionableSearch_Plot == 0 {
			err = serum.Error(wfapi.ECodeArgument,
				serum.WithMessageLiteral("file contained a plot document when a plot is not expected"),
			)
			return
		}
		capsule := wfapi.PlotCapsule{}
		_, e2 := ipld.UnmarshalStreaming(br, json.Decode, &capsule, wfapi.TypeSystem.TypeByName("PlotCapsule"))
		if e2 != nil {
			err = wfapi.ErrorSerialization("parsing what appeared to be a plot", err)
		} else {
			p = capsule.Plot
		}
		return
	case "formula":
		if accept&ActionableSearch_Formula == 0 {
			err = serum.Error(wfapi.ECodeArgument,
				serum.WithMessageLiteral("file contained a formula document when a formula is not expected"),
			)
			return
		}
		panic("not yet supported") // TODO FormulaAndContext is a little funky and might need reexamination.  Returning multiple values for that case is unpleasant.
	default:
		panic("unreachable")
	}
}
