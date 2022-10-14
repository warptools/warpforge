package workspace_test

import (
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	qt "github.com/frankban/quicktest"
	"github.com/warpfork/warpforge/pkg/workspace"
	"github.com/warpfork/warpforge/wfapi"
)

func TestWorkspaceCatalogPath(t *testing.T) {
	rootPath := "home/user"
	fsys := fstest.MapFS{
		"home/user/.warpforge": &fstest.MapFile{Mode: 0755 | fs.ModeDir},
	}
	ws, err := workspace.OpenWorkspace(fsys, rootPath)
	qt.Assert(t, err, qt.IsNil)
	for _, tt := range []struct {
		testCase string
		input    string
		output   string
		err      wfapi.Error
	}{
		{
			testCase: "empty catalog name",
			input:    "",
			output:   "home/user/.warpforge/catalog",
			err:      nil,
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
			err:      wfapi.ErrorCatalogInvalid("./foo", fmt.Sprintf("catalog name must match expression: %s", workspace.CatalogNameFormat)),
		},
		{
			testCase: "invalid relative path to parent",
			input:    "../",
			output:   "",
			err:      wfapi.ErrorCatalogInvalid("../", fmt.Sprintf("catalog name must match expression: %s", workspace.CatalogNameFormat)),
		},
		{
			testCase: "invalid relative path from parent",
			input:    "../foo",
			output:   "",
			err:      wfapi.ErrorCatalogInvalid("../foo", fmt.Sprintf("catalog name must match expression: %s", workspace.CatalogNameFormat)),
		},
		{
			testCase: "invalid only dot",
			input:    ".",
			output:   "",
			err:      wfapi.ErrorCatalogInvalid(".", fmt.Sprintf("catalog name must match expression: %s", workspace.CatalogNameFormat)),
		},
		{
			testCase: "invalid relative pwd",
			input:    "./",
			output:   "",
			err:      wfapi.ErrorCatalogInvalid("./", fmt.Sprintf("catalog name must match expression: %s", workspace.CatalogNameFormat)),
		},
		{
			testCase: "invalid separator",
			input:    "foo/bar",
			output:   "",
			err:      wfapi.ErrorCatalogInvalid("foo/bar", fmt.Sprintf("catalog name must match expression: %s", workspace.CatalogNameFormat)),
		},
		{
			testCase: "invalid length",
			input:    strings.Repeat("a", 64),
			output:   "",
			err:      wfapi.ErrorCatalogInvalid(strings.Repeat("a", 64), fmt.Sprintf("catalog name must match expression: %s", workspace.CatalogNameFormat)),
		},
		{
			testCase: "invalid leading slash",
			input:    "/foo",
			output:   "",
			err:      wfapi.ErrorCatalogInvalid("/foo", fmt.Sprintf("catalog name must match expression: %s", workspace.CatalogNameFormat)),
		},
		{
			testCase: "invalid only slash",
			input:    "/",
			output:   "",
			err:      wfapi.ErrorCatalogInvalid("/", fmt.Sprintf("catalog name must match expression: %s", workspace.CatalogNameFormat)),
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
