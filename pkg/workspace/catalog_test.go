package workspace

import (
	"testing"
	"testing/fstest"

	qt "github.com/frankban/quicktest"
	"github.com/warpfork/warpforge/wfapi"
)

func TestCatalogLookup(t *testing.T) {
	t.Run("happy-path", func(t *testing.T) {
		lineageData := `
		{
			"name": "foo",
			"metadata": {},
			"releases": [
				{
					"name": "bar",
					"metadata": {},
					"items": {
						"baz": "tar:abcd"
					}
				}
			] 
		}
		`
		mirrorData := `{
	"byWare": {
		"tar:abcd": [
			"https://example.com/foo/bar/baz.gz"
		]
	}
}
`

		fsys := fstest.MapFS{
			"home/user/.warpforge/catalog/foo/lineage.json": &fstest.MapFile{
				Mode: 0644,
				Data: []byte(lineageData),
			},
			"home/user/.warpforge/catalog/foo/mirrors.json": &fstest.MapFile{
				Mode: 0644,
				Data: []byte(mirrorData),
			},
		}
		ref := wfapi.CatalogRef{
			ModuleName:  "foo",
			ReleaseName: "bar",
			ItemName:    "baz",
		}

		t.Run("catalog-lineage", func(t *testing.T) {
			var err error
			wss, _, err := FindWorkspace(fsys, "", "home/user/")
			qt.Assert(t, err, qt.IsNil)

			l, err := wss.getCatalogLineage(ref)

			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, l.Name, qt.Equals, "foo")
		})
		t.Run("catalog-wareid", func(t *testing.T) {
			var err error
			wss, _, err := FindWorkspace(fsys, "", "home/user/")
			qt.Assert(t, err, qt.IsNil)

			wareId, _, err := wss.GetCatalogWare(ref)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, wareId.Hash, qt.Equals, "abcd")
			qt.Assert(t, wareId.Packtype, qt.Equals, wfapi.Packtype("tar"))

		})
	})
}
