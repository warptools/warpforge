package workspace_test

import (
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	qt "github.com/frankban/quicktest"

	"github.com/warptools/warpforge/pkg/workspace"
	"github.com/warptools/warpforge/wfapi"
)

func TestWorkspaceCatalogPath(t *testing.T) {
	rootPath := "home/user/workspace"
	fsys := fstest.MapFS{
		"home/user/.warpforge/root":      &fstest.MapFile{Mode: 0644 | fs.ModeDir},
		"home/user/workspace/.warpforge": &fstest.MapFile{Mode: 0644 | fs.ModeDir},
	}
	ws, err := workspace.OpenWorkspace(fsys, rootPath)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, ws.IsRootWorkspace(), qt.IsFalse)
	t.Run("empty catalog name", func(t *testing.T) {
		result, err := ws.CatalogPath("")
		qt.Check(t, err, qt.IsNil)
		qt.Check(t, result, qt.Equals, "home/user/workspace/.warpforge/catalog")
	})

	for _, tt := range []struct {
		testCase string
		input    string
	}{
		{
			testCase: "valid catalog name",
			input:    "foo",
		},
		{
			testCase: "invalid relative path",
			input:    "./foo",
		},
		{
			testCase: "invalid relative path to parent",
			input:    "../",
		},
		{
			testCase: "invalid relative path from parent",
			input:    "../foo",
		},
		{
			testCase: "invalid only dot",
			input:    ".",
		},
		{
			testCase: "invalid relative pwd",
			input:    "./",
		},
		{
			testCase: "invalid separator",
			input:    "foo/bar",
		},
		{
			testCase: "invalid length",
			input:    strings.Repeat("a", 64),
		},
		{
			testCase: "invalid leading slash",
			input:    "/foo",
		},
		{
			testCase: "invalid only slash",
			input:    "/",
		},
	} {
		t.Run(tt.testCase, func(t *testing.T) {
			result, err := ws.CatalogPath(tt.input)
			expectedErr := wfapi.ErrorCatalogName(tt.input, "named catalogs must be in a root workspace")
			qt.Check(t, err, qt.DeepEquals, expectedErr)
			qt.Check(t, result, qt.Equals, "")
		})
	}
}

func TestRootWorkspaceCatalogPathTestRootWorkspaceCatalogPath(t *testing.T) {
	rootPath := "home/user"
	fsys := fstest.MapFS{
		"home/user/.warpforge":      &fstest.MapFile{Mode: 0644 | fs.ModeDir},
		"home/user/.warpforge/root": &fstest.MapFile{Mode: 0644},
	}
	ws, err := workspace.OpenWorkspace(fsys, rootPath)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, ws.IsRootWorkspace(), qt.IsTrue)
	for _, tt := range []struct {
		testCase string
		input    string
		output   string
		err      wfapi.Error
	}{
		{
			testCase: "empty catalog name",
			input:    "",
			output:   "",
			err:      wfapi.ErrorCatalogName("", "catalogs for a root workspace must have a non-empty name"),
		},
		{
			testCase: "valid catalog name",
			input:    "foo",
			output:   "home/user/.warpforge/catalogs/foo",
			err:      nil,
		},
		{
			testCase: "invalid relative path",
			input:    "./foo",
			output:   "",
			err:      wfapi.ErrorCatalogName("./foo", fmt.Sprintf("catalog name must match expression: %s", workspace.CatalogNameFormat)),
		},
		{
			testCase: "invalid relative path to parent",
			input:    "../",
			output:   "",
			err:      wfapi.ErrorCatalogName("../", fmt.Sprintf("catalog name must match expression: %s", workspace.CatalogNameFormat)),
		},
		{
			testCase: "invalid relative path from parent",
			input:    "../foo",
			output:   "",
			err:      wfapi.ErrorCatalogName("../foo", fmt.Sprintf("catalog name must match expression: %s", workspace.CatalogNameFormat)),
		},
		{
			testCase: "invalid only dot",
			input:    ".",
			output:   "",
			err:      wfapi.ErrorCatalogName(".", fmt.Sprintf("catalog name must match expression: %s", workspace.CatalogNameFormat)),
		},
		{
			testCase: "invalid relative pwd",
			input:    "./",
			output:   "",
			err:      wfapi.ErrorCatalogName("./", fmt.Sprintf("catalog name must match expression: %s", workspace.CatalogNameFormat)),
		},
		{
			testCase: "invalid separator",
			input:    "foo/bar",
			output:   "",
			err:      wfapi.ErrorCatalogName("foo/bar", fmt.Sprintf("catalog name must match expression: %s", workspace.CatalogNameFormat)),
		},
		{
			testCase: "invalid length",
			input:    strings.Repeat("a", 64),
			output:   "",
			err:      wfapi.ErrorCatalogName(strings.Repeat("a", 64), fmt.Sprintf("catalog name must match expression: %s", workspace.CatalogNameFormat)),
		},
		{
			testCase: "invalid leading slash",
			input:    "/foo",
			output:   "",
			err:      wfapi.ErrorCatalogName("/foo", fmt.Sprintf("catalog name must match expression: %s", workspace.CatalogNameFormat)),
		},
		{
			testCase: "invalid only slash",
			input:    "/",
			output:   "",
			err:      wfapi.ErrorCatalogName("/", fmt.Sprintf("catalog name must match expression: %s", workspace.CatalogNameFormat)),
		},
	} {
		t.Run(tt.testCase, func(t *testing.T) {
			result, err := ws.CatalogPath(tt.input)
			if tt.err == nil {
				qt.Check(t, err, qt.IsNil)
			}
			if tt.err != nil {
				qt.Check(t, err, qt.DeepEquals, tt.err)
			}
			qt.Check(t, result, qt.Equals, tt.output)
		})
	}
}
