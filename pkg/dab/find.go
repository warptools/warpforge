package dab

import (
	"bufio"
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
	ActionableSearch_Formula ActionableSearch = 1 << iota // Use this bitflag to indicate a formula is an acceptable search result.
	ActionableSearch_Module                               // Use this bitflag to indicate a module is an acceptable search result.  Modules almost always have a plot associated with them, which will also be loaded by most search functions.
	ActionableSearch_Plot                                 // A module can always also have a plot; use this if a bare plot (no module) is acceptable.

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
//  - warpforge-error-invalid-argument -- when searchPath is absolute
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
	defer func() {
		if remainingSearchPath == "." {
			// canonicalize to an empty string for an empty remaining search path
			remainingSearchPath = ""
		}
	}()
	if filepath.IsAbs(searchPath) {
		err = serum.Error(wfapi.ECodeArgument,
			serum.WithMessageTemplate("search path {{path|q}} may not be an absolute path"),
			serum.WithDetail("path", searchPath),
		)
	}
	remainingSearchPath = searchPath

	// First round: this may be a file, and may be one of any of the types.
	// Stat it and consider that first:
	// if it is a file, that's its own whole detection procedure (and means no further search);
	// if it's not a file, we can proceed to the behavior for dir feature detection (without popping).
	foundPath = filepath.Join(basisPath, remainingSearchPath)
	fi, e2 := stat(fsys, foundPath)
	if e2 != nil && serum.Code(e2) != wfapi.ECodeMissing {
		err = e2
		return
	}
	if e2 == nil && fi.Mode().IsRegular() {
		remainingSearchPath = filepath.Dir(remainingSearchPath)
		fh, e2 := open(fsys, foundPath)
		defer fh.Close()
		if e2 != nil {
			err = e2
			return
		}
		m, p, f, err = guessActionableFromFile(fh, accept)
		if err != nil {
			return
		}
		if m != nil { // If a module was found, a plot may also exist as a sibling file.
			plotPath := filepath.Join(filepath.Dir(foundPath), MagicFilename_Plot)
			p, err = optionalPlotFromFile(fsys, plotPath)
			if err != nil {
				return
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
			foundPath = filepath.Join(basisPath, remainingSearchPath, MagicFilename_Module)
			m, p, err = optionalModulePlotFromFile(fsys, foundPath)
			if m != nil || err != nil {
				return
			}
		}
		// Peek for plot.
		if accept&ActionableSearch_Plot > 0 {
			foundPath = filepath.Join(basisPath, remainingSearchPath, MagicFilename_Plot)
			p, err = optionalPlotFromFile(fsys, foundPath)
			if p != nil || err != nil {
				return
			}
		}

		// Peek for Formula.
		// TODO

		// None of the filename peeks found a thing?
		// Okay.  If we can search further up, let's do so.
		foundPath = "" // clear found path
		if !searchUp {
			remainingSearchPath = filepath.Dir(remainingSearchPath)
			return
		}
		if len(remainingSearchPath) == 1 && (remainingSearchPath == "/" || remainingSearchPath == "." || remainingSearchPath == "") {
			return
		}
		remainingSearchPath = filepath.Dir(remainingSearchPath)
	}
}

// optionalModulePlotFromFile will load a module from file and an adjacent plot from file.
// Does not return an error for non-existent files.
func optionalModulePlotFromFile(fsys fs.FS, path string) (*wfapi.Module, *wfapi.Plot, error) {
	m, err := ModuleFromFile(fsys, path)
	if err != nil {
		if serum.Code(err) != wfapi.ECodeMissing { // notexist is ignored.
			// Any other error means our search has blind spots: error out.
			return nil, nil, wfapi.ErrorSearchingFilesystem("loading module", err)
		}
		return nil, nil, nil
	}
	// A module may also have a plot next to it; load that eagerly too.
	plotPath := filepath.Join(filepath.Dir(path), MagicFilename_Plot)
	p, err := optionalPlotFromFile(fsys, plotPath)
	if err != nil {
		return m, nil, err
	}
	return m, p, nil
}

// optionalPlotFromFile will load a plot from file.
// Does not return an error for non-existent files.
func optionalPlotFromFile(fsys fs.FS, path string) (*wfapi.Plot, error) {
	p, err := PlotFromFile(fsys, path)
	if err != nil {
		if serum.Code(err) != wfapi.ECodeMissing { // notexist is ignored.
			// Any error that's just just notexist: means our search has blind spots: error out.
			return nil, wfapi.ErrorSearchingFilesystem("modules, plots, or formulas", err)
		}
		return nil, nil
	}
	return p, nil
}

const (
	guessMagic_ModuleV1 = "module.v1"
	guessMagic_PlotV1   = "plot.v1"
	guessMagic_Formula  = "formula" // REVIEW: not sure why we don't use FormulaCapsule V1 value here
)

var guessMagic_All = []string{
	guessMagic_ModuleV1,
	guessMagic_PlotV1,
	guessMagic_Formula,
}

// guessActionableFromFile peeks at the first few bytes of a stream
// to guess whether it's a module, a plot, or a formula, and returns the right one.
func guessActionableFromFile(r io.Reader, accept ActionableSearch) (
	m *wfapi.Module, p *wfapi.Plot, f *wfapi.Formula, err error,
) {
	peekSize := 1024
	br := bufio.NewReaderSize(r, peekSize)
	peek, _ := br.Peek(peekSize)
	marker, err := GuessDocumentType(peek, guessMagic_All)
	if err != nil {
		return
	}
	switch marker {
	case guessMagic_ModuleV1:
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
	case guessMagic_PlotV1:
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
	case guessMagic_Formula:
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
