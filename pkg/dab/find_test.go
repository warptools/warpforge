package dab

import (
	"io/fs"
	"math/rand"
	"path/filepath"
	"testing"
	"testing/fstest"

	qt "github.com/frankban/quicktest"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
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

const Mode = 0444

// randIrregular returns a fs.FileMode from modes in fs.ModeType except for ModeSymlink because stat/open will follow links
func randIrregular() fs.FileMode {
	modes := []fs.FileMode{fs.ModeDir, fs.ModeSymlink, fs.ModeNamedPipe, fs.ModeSocket, fs.ModeDevice, fs.ModeCharDevice, fs.ModeIrregular}
	n := rand.Intn(len(modes))
	return modes[n]
}

func TestFindActionableFromFS_NotFound(t *testing.T) {
	t.Run("when no actionable files exist", func(t *testing.T) {
		fsys := fstest.MapFS{}
		m, p, f, path, rem, err := FindActionableFromFS(fsys, "", "non/existing/directory", true, ActionableSearch_Any)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, m, qt.IsNil)
		qt.Assert(t, p, qt.IsNil)
		qt.Assert(t, f, qt.IsNil)
		qt.Assert(t, path, qt.Equals, "")
		qt.Assert(t, rem, qt.Equals, "")
	})
	t.Run("when actionable files exist above basis directory and searchUp is true", func(t *testing.T) {
		fsys := fstest.MapFS{
			filepath.Join("foo", MagicFilename_Module):  &fstest.MapFile{Mode: Mode, Data: ModuleData},
			filepath.Join("foo", MagicFilename_Plot):    &fstest.MapFile{Mode: Mode, Data: PlotData},
			filepath.Join("foo", MagicFilename_Formula): &fstest.MapFile{Mode: Mode, Data: FormulaData},
		}
		m, p, f, path, rem, err := FindActionableFromFS(fsys, "foo/bar", "non/existing/directory", true, ActionableSearch_Any)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, m, qt.IsNil)
		qt.Assert(t, p, qt.IsNil)
		qt.Assert(t, f, qt.IsNil)
		qt.Assert(t, path, qt.Equals, "")
		qt.Assert(t, rem, qt.Equals, "")
	})
	t.Run("when actionable files exist above search directory and searchUp is false", func(t *testing.T) {
		fsys := fstest.MapFS{
			filepath.Join("foo", MagicFilename_Module):  &fstest.MapFile{Mode: Mode, Data: ModuleData},
			filepath.Join("foo", MagicFilename_Plot):    &fstest.MapFile{Mode: Mode, Data: PlotData},
			filepath.Join("foo", MagicFilename_Formula): &fstest.MapFile{Mode: Mode, Data: FormulaData},
		}
		m, p, f, path, rem, err := FindActionableFromFS(fsys, "", "foo/bar", false, ActionableSearch_Any)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, m, qt.IsNil)
		qt.Assert(t, p, qt.IsNil)
		qt.Assert(t, f, qt.IsNil)
		qt.Assert(t, path, qt.Equals, "")
		qt.Assert(t, rem, qt.Equals, "foo")
	})
	t.Run("when direct file is not regular", func(t *testing.T) {
		fsys := fstest.MapFS{
			filepath.Join("foo", MagicFilename_Module):  &fstest.MapFile{Mode: Mode | randIrregular(), Data: ModuleData},
			filepath.Join("foo", MagicFilename_Plot):    &fstest.MapFile{Mode: Mode | randIrregular(), Data: PlotData},
			filepath.Join("foo", MagicFilename_Formula): &fstest.MapFile{Mode: Mode | randIrregular(), Data: FormulaData},
		}
		for path := range fsys {
			path := path
			name := filepath.Base(path)
			t.Run(name, func(t *testing.T) {
				// search Up must be false for this test otherwise it will open via *FromFile
				m, p, f, path, rem, err := FindActionableFromFS(fsys, "", path, false, ActionableSearch_Any)
				qt.Assert(t, err, qt.IsNil)
				qt.Assert(t, m, qt.IsNil)
				qt.Assert(t, p, qt.IsNil)
				qt.Assert(t, f, qt.IsNil)
				qt.Assert(t, path, qt.Equals, "")
				qt.Assert(t, rem, qt.Equals, "foo")
			})
		}
	})
}

// test the priority order of FindActionableFromFS
// module > plot > formula
// Also tests behavior where the files exist in the filesystem root directory.
func TestFindActionableFromFS_Priority(t *testing.T) {
	t.Run("highest", func(t *testing.T) {
		fsys := fstest.MapFS{
			MagicFilename_Module:  &fstest.MapFile{Mode: Mode, Data: ModuleData},
			MagicFilename_Plot:    &fstest.MapFile{Mode: Mode, Data: PlotData},
			MagicFilename_Formula: &fstest.MapFile{Mode: Mode, Data: FormulaData},
		}
		m, p, f, path, rem, err := FindActionableFromFS(fsys, "", "", false, ActionableSearch_Any)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, m, qt.IsNotNil)
		qt.Assert(t, p, qt.IsNotNil)
		qt.Assert(t, f, qt.IsNil)
		qt.Assert(t, path, qt.Equals, MagicFilename_Module)
		qt.Assert(t, rem, qt.Equals, "")
	})
	t.Run("mid", func(t *testing.T) {
		fsys := fstest.MapFS{
			MagicFilename_Plot:    &fstest.MapFile{Mode: Mode, Data: PlotData},
			MagicFilename_Formula: &fstest.MapFile{Mode: Mode, Data: FormulaData},
		}
		m, p, f, path, rem, err := FindActionableFromFS(fsys, "", "", false, ActionableSearch_Any)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, m, qt.IsNil)
		qt.Assert(t, p, qt.IsNotNil)
		qt.Assert(t, f, qt.IsNil)
		qt.Assert(t, path, qt.Equals, MagicFilename_Plot)
		qt.Assert(t, rem, qt.Equals, "")
	})
	t.Run("low", func(t *testing.T) {
		fsys := fstest.MapFS{
			MagicFilename_Formula: &fstest.MapFile{Mode: Mode, Data: FormulaData},
		}
		m, p, f, path, rem, err := FindActionableFromFS(fsys, "", "", false, ActionableSearch_Any)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, m, qt.IsNil)
		qt.Assert(t, p, qt.IsNil)
		qt.Assert(t, f, qt.Not(qt.IsNotNil))                         // TODO: formula loading not implemented
		qt.Assert(t, path, qt.Not(qt.Equals), MagicFilename_Formula) // TODO: formula loading not implemented
		qt.Assert(t, rem, qt.Equals, "")
	})
}

func TestFindActionableFromFS_DirectFile(t *testing.T) {
	basePath := "path/to/files"
	modulePath := filepath.Join(basePath, MagicFilename_Module)
	plotPath := filepath.Join(basePath, MagicFilename_Plot)
	frmPath := filepath.Join(basePath, MagicFilename_Formula)
	fsys := fstest.MapFS{
		modulePath: &fstest.MapFile{Mode: Mode, Data: ModuleData},
		plotPath:   &fstest.MapFile{Mode: Mode, Data: PlotData},
		frmPath:    &fstest.MapFile{Mode: Mode, Data: FormulaData},
	}
	t.Run("module with plot", func(t *testing.T) {
		m, p, f, path, rem, err := FindActionableFromFS(fsys, "", modulePath, true, ActionableSearch_Any)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, m, qt.IsNotNil)
		qt.Assert(t, p, qt.IsNotNil)
		qt.Assert(t, f, qt.IsNil)
		qt.Assert(t, rem, qt.Equals, filepath.Dir(modulePath))
		qt.Assert(t, path, qt.Equals, modulePath)
	})
	t.Run("module no plot", func(t *testing.T) {
		fsys := fstest.MapFS{
			modulePath: &fstest.MapFile{Mode: Mode, Data: ModuleData},
		}
		m, p, f, path, rem, err := FindActionableFromFS(fsys, "", modulePath, true, ActionableSearch_Any)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, m, qt.IsNotNil)
		qt.Assert(t, p, qt.IsNil)
		qt.Assert(t, f, qt.IsNil)
		qt.Assert(t, rem, qt.Equals, filepath.Dir(modulePath))
		qt.Assert(t, path, qt.Equals, modulePath)
	})
	t.Run("plot", func(t *testing.T) {
		m, p, f, path, rem, err := FindActionableFromFS(fsys, "", plotPath, true, ActionableSearch_Any)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, m, qt.IsNil)
		qt.Assert(t, p, qt.IsNotNil)
		qt.Assert(t, f, qt.IsNil)
		qt.Assert(t, rem, qt.Equals, filepath.Dir(plotPath))
		qt.Assert(t, path, qt.Equals, plotPath)
	})
	t.Run("formula", func(t *testing.T) {
		qt.Assert(t, func() {
			FindActionableFromFS(fsys, "", frmPath, true, ActionableSearch_Any)
		}, qt.PanicMatches, "unreachable")
		t.Skipf("TODO: unimplemented")

		m, p, f, path, rem, err := FindActionableFromFS(fsys, "", frmPath, true, ActionableSearch_Any)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, m, qt.IsNil)
		qt.Assert(t, p, qt.IsNil)
		qt.Assert(t, f, qt.IsNotNil)
		qt.Assert(t, rem, qt.Equals, filepath.Dir(frmPath))
		qt.Assert(t, path, qt.Equals, frmPath)
	})
	t.Run("module missing; search up", func(t *testing.T) {
		searchPath := filepath.Join(filepath.Dir(modulePath), "subdir", MagicFilename_Module)
		m, p, f, path, rem, err := FindActionableFromFS(fsys, "", searchPath, true, ActionableSearch_Any)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, m, qt.IsNotNil)
		qt.Assert(t, p, qt.IsNotNil)
		qt.Assert(t, f, qt.IsNil)
		qt.Assert(t, rem, qt.Equals, filepath.Dir(modulePath))
		qt.Assert(t, path, qt.Equals, modulePath)
	})
}
