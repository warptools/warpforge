package workspace

import (
	"io/fs"
	"testing"
	"testing/fstest"

	qt "github.com/frankban/quicktest"
)

func TestOpenWorkspace(t *testing.T) {
	fsys := fstest.MapFS{
		"test/.warpforge/root": &fstest.MapFile{Mode: 0755},
		"test/.warpforge":      &fstest.MapFile{Mode: 0755 | fs.ModeDir},
	}
	_, err := fs.Stat(fsys, "test/.warpforge/root")
	qt.Assert(t, err, qt.IsNil)
	isRoot := checkIsRootWorkspace(fsys, "/test")
	qt.Assert(t, isRoot, qt.IsTrue, qt.Commentf("checkIsRootWorkspace is incorrect"))
	ws := openWorkspace(fsys, "/test")
	qt.Assert(t, ws, qt.IsNotNil)
	qt.Assert(t, ws.IsRootWorkspace(), qt.IsTrue, qt.Commentf("openWorkspace is incorrect"))
}
