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

func TestOpenHomeWorkspace(t *testing.T) {
	ws, err := workspace.OpenHomeWorkspace(fstest.MapFS{})
	qt.Assert(t, err, qt.IsNotNil)
	qt.Assert(t, ws, qt.IsNil)

	fsys := fstest.MapFS{
		"home/user/.warphome": &fstest.MapFile{Mode: 0644 | fs.ModeDir},
	}
	t.Log(fsys)
	ws, err = workspace.OpenHomeWorkspace(fsys)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, ws.InternalPath(), qt.Equals, "home/user/.warphome")
}

func TestLocalWorkspaceCatalogPath(t *testing.T) {
	localPath := "test-workspace/local"
	fsys := fstest.MapFS{
		"test-workspace/.warpforge/root":  &fstest.MapFile{Mode: 0644 | fs.ModeDir},
		"test-workspace/.warpforge":       &fstest.MapFile{Mode: 0644 | fs.ModeDir},
		"test-workspace/local/.warpforge": &fstest.MapFile{Mode: 0644 | fs.ModeDir},
	}
	ws, err := workspace.OpenWorkspace(fsys, localPath)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, ws.IsRootWorkspace(), qt.IsFalse)
	qt.Assert(t, ws.IsHomeWorkspace(), qt.IsFalse)
	t.Run("empty catalog name", func(t *testing.T) {
		result, err := ws.CatalogPath("")
		qt.Check(t, err, qt.IsNil)
		qt.Check(t, result, qt.Equals, "test-workspace/local/.warpforge/catalog")
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
			tt := tt
			result, err := ws.CatalogPath(tt.input)
			expectedErr := wfapi.ErrorCatalogName(tt.input, "named catalogs must be in a root workspace")
			qt.Check(t, err, qt.DeepEquals, expectedErr)
			qt.Check(t, result, qt.Equals, "")
		})
	}
}

func TestRootWorkspaceCatalogPath(t *testing.T) {
	rootPath := "test-workspace"
	fsys := fstest.MapFS{
		"test-workspace/.warpforge":      &fstest.MapFile{Mode: 0644 | fs.ModeDir},
		"test-workspace/.warpforge/root": &fstest.MapFile{Mode: 0644},
	}
	ws, err := workspace.OpenWorkspace(fsys, rootPath)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, ws.IsRootWorkspace(), qt.IsTrue)
	qt.Assert(t, ws.IsHomeWorkspace(), qt.IsFalse)
	for _, tt := range []struct {
		testCase string
		input    string
		output   string
		err      error
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
			output:   "test-workspace/.warpforge/catalogs/foo",
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

func TestListCatalogs(t *testing.T) {
	rootPath := "test-workspace"
	fsys := fstest.MapFS{
		"test-workspace/.warpforge":                             &fstest.MapFile{Mode: 0644 | fs.ModeDir},
		"test-workspace/.warpforge/root":                        &fstest.MapFile{Mode: 0644},
		"test-workspace/.warpforge/catalogs":                    &fstest.MapFile{Mode: 0644 | fs.ModeDir},
		"test-workspace/.warpforge/catalogs/somedir":            &fstest.MapFile{Mode: 0644 | fs.ModeDir},
		"test-workspace/.warpforge/catalogs/symlink":            &fstest.MapFile{Mode: 0644 | fs.ModeSymlink},
		"test-workspace/.warpforge/catalogs/just-a-file-counts": &fstest.MapFile{Mode: 0644},
		"test-workspace/.warpforge/catalogs/_not_a_catalog":     &fstest.MapFile{Mode: 0644 | fs.ModeDir},
		"test-workspace/.warpforge/catalogs/1-still-a.catalog":  &fstest.MapFile{Mode: 0644 | fs.ModeDir},
	}
	ws, err := workspace.OpenWorkspace(fsys, rootPath)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, ws.IsRootWorkspace(), qt.IsTrue)
	qt.Assert(t, ws.IsHomeWorkspace(), qt.IsFalse)
	result, err := ws.ListCatalogs()
	qt.Check(t, err, qt.IsNil)
	expected := []string{
		"somedir", "symlink", "just-a-file-counts", "1-still-a.catalog",
	}
	qt.Check(t, result, qt.ContentEquals, expected)
}
