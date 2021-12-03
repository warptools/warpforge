package workspace

import (
	"testing"
	"testing/fstest"

	qt "github.com/frankban/quicktest"
	"github.com/warpfork/warpforge/wfapi"
)

func TestCatalogLookup(t *testing.T) {
	t.Run("catalog-lookup", func(t *testing.T) {
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

		ref := wfapi.CatalogRef{
			ModuleName:  "foo",
			ReleaseName: "bar",
			ItemName:    "baz",
		}

		t.Run("single-catalog-lookup", func(t *testing.T) {
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
			var err error
			wss, _, err := FindWorkspace(fsys, "", "home/user/")
			qt.Assert(t, err, qt.IsNil)

			wareId, wareAddr, err := wss.GetCatalogWare(ref)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, wareId.Hash, qt.Equals, "abcd")
			qt.Assert(t, wareId.Packtype, qt.Equals, wfapi.Packtype("tar"))
			qt.Assert(t, *wareAddr, qt.Equals, wfapi.WarehouseAddr("https://example.com/foo/bar/baz.gz"))

		})
		t.Run("multi-catalog-lookup", func(t *testing.T) {
			fsys := fstest.MapFS{
				"home/user/.warpforge/catalogs/test/foo/lineage.json": &fstest.MapFile{
					Mode: 0644,
					Data: []byte(lineageData),
				},
				"home/user/.warpforge/catalogs/test/foo/mirrors.json": &fstest.MapFile{
					Mode: 0644,
					Data: []byte(mirrorData),
				},
			}
			var err error
			wss, _, err := FindWorkspace(fsys, "", "home/user/")
			qt.Assert(t, err, qt.IsNil)

			wareId, wareAddr, err := wss.GetCatalogWare(ref)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, wareId.Hash, qt.Equals, "abcd")
			qt.Assert(t, wareId.Packtype, qt.Equals, wfapi.Packtype("tar"))
			qt.Assert(t, *wareAddr, qt.Equals, wfapi.WarehouseAddr("https://example.com/foo/bar/baz.gz"))
		})
	})
}
