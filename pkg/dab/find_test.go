package dab

import (
	"context"
	"io/fs"
	"math/rand"
	"path/filepath"
	"regexp"
	"testing"
	"testing/fstest"

	qt "github.com/frankban/quicktest"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"

	"github.com/warptools/warpforge/pkg/testutil/turtletb"
	"github.com/warptools/warpforge/wfapi"
)

func MustMarshal(i interface{}, typeName string) []byte {
	data, err := ipld.Marshal(json.Encode, i, wfapi.TypeSystem.TypeByName(typeName))
	if err != nil {
		panic(err)
	}
	return data
}

// Data blocks required for "GuessDocumentType"
var (
	ModuleData  = MustMarshal(&wfapi.ModuleCapsule{Module: &wfapi.Module{Name: "hello"}}, "ModuleCapsule")
	PlotData    = MustMarshal(&wfapi.PlotCapsule{Plot: &wfapi.Plot{}}, "PlotCapsule")
	FormulaData = MustMarshal(&wfapi.Formula{Action: wfapi.Action{Echo: &wfapi.Action_Echo{}}}, "Formula")
)

const testDefaultMode = 0444

// randIrregular returns a fs.FileMode from modes in fs.ModeType except for ModeSymlink because stat/open will follow links
func randIrregular() fs.FileMode {
	modes := []fs.FileMode{fs.ModeDir, fs.ModeSymlink, fs.ModeNamedPipe, fs.ModeSocket, fs.ModeDevice, fs.ModeCharDevice, fs.ModeIrregular}
	n := rand.Intn(len(modes))
	return modes[n]
}

type fafsInput struct {
	fs.FS
	basis    string
	search   string
	searchUp bool
	mode     ActionableSearch
}

type fafsOutput struct {
	expectFailure []turtletb.Record // Value will be a regex pattern to match
	panicPattern  string
	module        bool
	plot          bool
	formula       bool
	path          string
	remain        string
	error         bool
}

type testcaseFindActionableFromFS struct {
	name    string
	inputs  fafsInput
	outputs fafsOutput
}

func (tt *testcaseFindActionableFromFS) run(t *testing.T) {
	in := tt.inputs
	fsys := in.FS
	if fsys == nil {
		fsys = fstest.MapFS{}
	}
	if len(tt.outputs.panicPattern) > 0 {
		qt.Assert(t, func() {
			FindActionableFromFS(fsys, in.basis, in.search, in.searchUp, in.mode)
		}, qt.PanicMatches, tt.outputs.panicPattern)
		t.Skipf("expected panic caught")
	}
	m, p, f, path, rem, err := FindActionableFromFS(fsys, in.basis, in.search, in.searchUp, in.mode)
	isNotNil := func(b bool) qt.Checker {
		if b {
			return qt.IsNotNil
		}
		return qt.IsNil
	}
	out := tt.outputs

	doAssertions := func(t testing.TB) {
		qt.Assert(t, err, isNotNil(out.error))
		qt.Assert(t, m, isNotNil(out.module))
		qt.Assert(t, p, isNotNil(out.plot))
		qt.Assert(t, f, isNotNil(out.formula))
		qt.Assert(t, path, qt.Equals, out.path)
		qt.Assert(t, rem, qt.Equals, out.remain)
	}
	if len(out.expectFailure) > 0 {
		tb := turtletb.TB{TB: t}
		ctx := context.Background()
		ctx = tb.Start(ctx, doAssertions)
		<-ctx.Done()
		for _, expect := range out.expectFailure {
			found := false
			re, err := regexp.Compile(expect.Value)
			qt.Assert(t, err, qt.IsNil, qt.Commentf("expected failure pattern must compile: %s", expect.Value))
			for _, r := range tb.Records {
				if expect.Kind&r.Kind == 0 {
					// record is not an expected kind
					continue
				}
				if re.MatchString(r.Value) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("Expected test to produce %q record matching %q but did not", expect.Kind, expect.Value)
			}
		}
		qt.Assert(t, tb.Failed(), qt.IsTrue, qt.Commentf("test is expected to fail but did not"))
		return
	}
	doAssertions(t)
}

type testfsOpt func(m fstest.MapFS) fstest.MapFS

func testFSRandIrregularMode(m fstest.MapFS) fstest.MapFS {
	for k, v := range m {
		v.Mode = testDefaultMode | randIrregular()
		m[k] = v
	}
	return m
}

func newtestFS(basis string, files []string, opts ...testfsOpt) fstest.MapFS {
	fsys := fstest.MapFS{}
	data := func(f string) []byte {
		switch f {
		case MagicFilename_Module:
			return ModuleData
		case MagicFilename_Plot:
			return PlotData
		case MagicFilename_Formula:
			return FormulaData
		}
		return []byte{}
	}
	for _, f := range files {
		fsys[filepath.Join(basis, f)] = &fstest.MapFile{Mode: testDefaultMode, Data: data(f)}
	}
	for _, o := range opts {
		fsys = o(fsys)
	}
	return fsys
}

func merge[K comparable, V any](maps ...map[K]V) map[K]V {
	size := 0
	for _, m := range maps {
		size += len(m)
	}
	r := make(map[K]V, size)
	for _, m := range maps {
		for k, v := range m {
			r[k] = v
		}
	}
	return r
}

func TestFindActionableFromFS_NotFound(t *testing.T) {
	fsMPF := newtestFS("foo", []string{MagicFilename_Module, MagicFilename_Plot, MagicFilename_Formula})
	fsIrregularMPF := newtestFS("foo", []string{MagicFilename_Module, MagicFilename_Plot, MagicFilename_Formula}, testFSRandIrregularMode)
	for _, tt := range []testcaseFindActionableFromFS{
		{name: "when no actionable files exist",
			inputs:  fafsInput{basis: "", search: "non/existing/directory", searchUp: true, mode: ActionableSearch_Any},
			outputs: fafsOutput{module: false, plot: false, formula: false, path: "", remain: "", error: false},
		},
		{name: "when actionable files exist above basis directory and searchUp is true",
			inputs:  fafsInput{FS: fsMPF, basis: "foo/bar", search: "non/existing/directory", searchUp: true, mode: ActionableSearch_Any},
			outputs: fafsOutput{module: false, plot: false, formula: false, path: "", remain: "", error: false},
		},
		{name: "when actionable files exist above search directory and searchUp is false",
			inputs:  fafsInput{FS: fsMPF, basis: "", search: "foo/bar", searchUp: false, mode: ActionableSearch_Any},
			outputs: fafsOutput{module: false, plot: false, formula: false, path: "", remain: "foo", error: false},
		},
		{name: "when direct file is not regular; module",
			inputs:  fafsInput{FS: fsIrregularMPF, basis: "", search: "foo/module.wf", searchUp: false, mode: ActionableSearch_Any},
			outputs: fafsOutput{module: false, plot: false, formula: false, path: "", remain: "foo", error: false},
		},
		{name: "when direct file is not regular; plot",
			inputs:  fafsInput{FS: fsIrregularMPF, basis: "", search: "foo/plot.wf", searchUp: false, mode: ActionableSearch_Any},
			outputs: fafsOutput{module: false, plot: false, formula: false, path: "", remain: "foo", error: false},
		},
		{name: "when direct file is not regular; formula",
			inputs:  fafsInput{FS: fsIrregularMPF, basis: "", search: "foo/formula.wf", searchUp: false, mode: ActionableSearch_Any},
			outputs: fafsOutput{module: false, plot: false, formula: false, path: "", remain: "foo", error: false},
		},
	} {
		tt := tt
		t.Run(tt.name, tt.run)
	}
}

// formulaNotImplementedError is used to check for the expected error caused by formula loading not being implemented
var formulaNotImplementedError = []turtletb.Record{{Kind: turtletb.RK_Error, Value: `(?m)got:\s*\(\*wfapi\.Formula\)\(nil\)\s*stack`}}

// test the priority order of FindActionableFromFS
// module > plot > formula
// Also tests behavior where the files exist in the filesystem root directory.
func TestFindActionableFromFS_Priority(t *testing.T) {
	fsMPF := newtestFS("", []string{MagicFilename_Module, MagicFilename_Plot, MagicFilename_Formula})
	fsPF := newtestFS("", []string{MagicFilename_Plot, MagicFilename_Formula})
	fsF := newtestFS("", []string{MagicFilename_Formula})
	for _, tt := range []testcaseFindActionableFromFS{
		{name: "highest",
			inputs:  fafsInput{FS: fsMPF, basis: "", search: "", searchUp: true, mode: ActionableSearch_Any},
			outputs: fafsOutput{module: true, plot: true, formula: false, path: MagicFilename_Module, remain: "", error: false},
		},
		{name: "mid",
			inputs:  fafsInput{FS: fsPF, basis: "", search: "", searchUp: true, mode: ActionableSearch_Any},
			outputs: fafsOutput{module: false, plot: true, formula: false, path: MagicFilename_Plot, remain: "", error: false},
		},
		{name: "low",
			inputs: fafsInput{FS: fsF, basis: "", search: "", searchUp: true, mode: ActionableSearch_Any},
			outputs: fafsOutput{expectFailure: formulaNotImplementedError, // "FIXME: formula loading is not implemented",
				module: false, plot: false, formula: true, path: MagicFilename_Formula, remain: "", error: false},
		},
	} {
		tt := tt
		t.Run(tt.name, tt.run)
	}
}

func TestFindActionableFromFS(t *testing.T) {
	fsRootMPF := newtestFS("", []string{MagicFilename_Module, MagicFilename_Plot, MagicFilename_Formula})
	fsParentMPF := newtestFS("path", []string{MagicFilename_Module, MagicFilename_Plot, MagicFilename_Formula})
	fsMPF := newtestFS("path/to/files", []string{MagicFilename_Module, MagicFilename_Plot, MagicFilename_Formula})
	fsMPF = merge(fsMPF, fsRootMPF, fsParentMPF) // merge these to test that we get the correct one
	fsMF := newtestFS("path/to/files", []string{MagicFilename_Module, MagicFilename_Formula})

	for _, tt := range []testcaseFindActionableFromFS{
		{name: "direct module with plot",
			inputs:  fafsInput{FS: fsMPF, basis: "", search: "path/to/files/module.wf", searchUp: true, mode: ActionableSearch_Any},
			outputs: fafsOutput{module: true, plot: true, formula: false, path: "path/to/files/module.wf", remain: "path/to/files", error: false},
		},
		{name: "direct module without plot",
			inputs:  fafsInput{FS: fsMF, basis: "", search: "path/to/files/module.wf", searchUp: true, mode: ActionableSearch_Any},
			outputs: fafsOutput{module: true, plot: false, formula: false, path: "path/to/files/module.wf", remain: "path/to/files", error: false},
		},
		{name: "direct plot",
			inputs:  fafsInput{FS: fsMPF, basis: "", search: "path/to/files/plot.wf", searchUp: true, mode: ActionableSearch_Any},
			outputs: fafsOutput{module: false, plot: true, formula: false, path: "path/to/files/plot.wf", remain: "path/to/files", error: false},
		},
		{name: "direct formula",
			inputs: fafsInput{FS: fsMPF, basis: "", search: "path/to/files/formula.wf", searchUp: true, mode: ActionableSearch_Any},
			outputs: fafsOutput{
				panicPattern: "unreachable", // FIXME: formula loading is not implemented
				module:       false, plot: false, formula: true, path: "path/to/files/formula.wf", remain: "path/to/files", error: false},
		},
		{name: "find module in parent directories when search path is a direct file",
			inputs:  fafsInput{FS: fsMPF, basis: "", search: "path/to/files/subdir/module.wf", searchUp: true, mode: ActionableSearch_Any},
			outputs: fafsOutput{module: true, plot: true, formula: false, path: "path/to/files/module.wf", remain: "path/to/files", error: false},
		},
		{name: "find module and plot when search contains trailing slash",
			inputs:  fafsInput{FS: fsMPF, basis: "", search: "path/to/files/subdir/", searchUp: true, mode: ActionableSearch_Any},
			outputs: fafsOutput{module: true, plot: true, formula: false, path: "path/to/files/module.wf", remain: "path/to/files", error: false},
		},
		{name: "find module and plot with exact search path",
			inputs:  fafsInput{FS: fsMPF, basis: "", search: "path/to/files", searchUp: true, mode: ActionableSearch_Any},
			outputs: fafsOutput{module: true, plot: true, formula: false, path: "path/to/files/module.wf", remain: "path/to/files", error: false},
		},
		{name: "find module and plot with exact basis path",
			inputs:  fafsInput{FS: fsMPF, basis: "path/to/files", search: "", searchUp: true, mode: ActionableSearch_Any},
			outputs: fafsOutput{module: true, plot: true, formula: false, path: "path/to/files/module.wf", remain: "", error: false},
		},
		{name: "find module and plot with only module filter",
			inputs:  fafsInput{FS: fsMPF, basis: "path/to/files", search: "", searchUp: true, mode: ActionableSearch_Module},
			outputs: fafsOutput{module: true, plot: true, formula: false, path: "path/to/files/module.wf", remain: "", error: false},
		},
		{name: "find plot with filter",
			inputs:  fafsInput{FS: fsMPF, basis: "path/to/files", search: "", searchUp: true, mode: ActionableSearch_Plot},
			outputs: fafsOutput{module: false, plot: true, formula: false, path: "path/to/files/plot.wf", remain: "", error: false},
		},
		{name: "find formula with filter",
			inputs: fafsInput{FS: fsMPF, basis: "path/to/files", search: "", searchUp: true, mode: ActionableSearch_Formula},
			outputs: fafsOutput{expectFailure: formulaNotImplementedError, // "FIXME: formula loading is not implemented",
				module: false, plot: false, formula: true, path: "path/to/files/formula.wf", remain: "", error: false},
		},
	} {
		tt := tt
		t.Run(tt.name, tt.run)
	}
}
