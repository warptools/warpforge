package workspace

import (
	"testing"
	"testing/fstest"

	qt "github.com/frankban/quicktest"
	"github.com/warpfork/warpforge/wfapi"
)

func TestCatalogLookup(t *testing.T) {
	t.Run("catalog-lookup", func(t *testing.T) {
		moduleData := `{
	"catalogmodule.v1": {
		"name": "example.com/module",
		"metadata": {},
		"releases": {
			"v1.0": "zM5K3YdpMrp1z7Zs2yMQbmRxndxeCbk7LeCqRzgBcC64JTLNSyGnJtwUdim94mddgbFNw2s"
		}
	}
}
`
		releaseData := `{
	"name": "v1.0",
	"metadata": {
		"replay": "zM5K3YuaVZAyaNzXnof5ixD5DW68WjuZYdgMduZJz75p9kxvgk8Mb3Hq8e6GMozwuSn7VF7"
	},
	"items": {
		"x86_64": "tar:abcd"
	} 
}
`
		mirrorData := `{
	"catalogmirrors.v1": {
		"byWare": {
			"tar:abcd": [
				"https://example.com/module/module-v1.0-x86_64.tgz"
			]
		}
	}
}
`
		replayData := `{
	"plot.v1": {
		"inputs": {
				"rootfs": "catalog:alpinelinux.org/alpine:v3.15.0:x86_64"
		},
		"steps": {
				"hello-world": {
						"protoformula": {
								"inputs": {
										"/": "pipe::rootfs"
								},
								"action": {
										"script": {
												"interpreter": "/bin/sh",
												"contents": [
														"mkdir /output",
														"echo 'hello world' | tee /output/file"
												],
												"network": false
										}
								},
								"outputs": {
										"out": {
												"from": "/output",
												"packtype": "tar"
										}
								}
						}
				}
		},
		"outputs": {
				"output": "pipe:hello-world:out"
		}
	}
}
`

		ref := wfapi.CatalogRef{
			ModuleName:  "example.com/module",
			ReleaseName: "v1.0",
			ItemName:    "x86_64",
		}

		t.Run("single-catalog-lookup", func(t *testing.T) {
			fsys := fstest.MapFS{
				"home/user/.warpforge/catalog/example.com/module/module.json": &fstest.MapFile{
					Mode: 0644,
					Data: []byte(moduleData),
				},
				"home/user/.warpforge/catalog/example.com/module/module.releases/v1.0.json": &fstest.MapFile{
					Mode: 0644,
					Data: []byte(releaseData),
				},
				"home/user/.warpforge/catalog/example.com/module/mirrors.json": &fstest.MapFile{
					Mode: 0644,
					Data: []byte(mirrorData),
				},
			}
			var err error
			wss, _, err := FindWorkspace(fsys, "", "home/user/")
			qt.Assert(t, err, qt.IsNil)

			wareId, wareAddr, err := wss.GetCatalogWare(ref)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, wareId, qt.IsNotNil)
			qt.Assert(t, wareId.Hash, qt.Equals, "abcd")
			qt.Assert(t, wareId.Packtype, qt.Equals, wfapi.Packtype("tar"))
			qt.Assert(t, *wareAddr, qt.Equals, wfapi.WarehouseAddr("https://example.com/module/module-v1.0-x86_64.tgz"))

		})
		t.Run("multi-catalog-lookup", func(t *testing.T) {
			fsys := fstest.MapFS{
				"home/user/.warpforge/catalogs/test/example.com/module/module.json": &fstest.MapFile{
					Mode: 0644,
					Data: []byte(moduleData),
				},
				"home/user/.warpforge/catalogs/test/example.com/module/module.releases/v1.0.json": &fstest.MapFile{
					Mode: 0644,
					Data: []byte(releaseData),
				},
				"home/user/.warpforge/catalogs/test/example.com/module/mirrors.json": &fstest.MapFile{
					Mode: 0644,
					Data: []byte(mirrorData),
				},
				"home/user/.warpforge/catalogs/test/example.com/module-two/module.json": &fstest.MapFile{
					Mode: 0644,
					Data: []byte(moduleData),
				},
				"home/user/.warpforge/catalogs/test/example.com/module-two/module.releases/v1.0.json": &fstest.MapFile{
					Mode: 0644,
					Data: []byte(releaseData),
				},
				"home/user/.warpforge/catalogs/test/example.com/module-two/mirrors.json": &fstest.MapFile{
					Mode: 0644,
					Data: []byte(mirrorData),
				},
			}
			var err error
			ws, _, err := FindWorkspace(fsys, "", "home/user/")
			qt.Assert(t, err, qt.IsNil)

			catName := "test"
			cat, err := ws.OpenCatalog(&catName)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, len(cat.Modules()), qt.Equals, 2)
			qt.Assert(t, cat.Modules()[0], qt.Equals, wfapi.ModuleName("example.com/module"))
			qt.Assert(t, cat.Modules()[1], qt.Equals, wfapi.ModuleName("example.com/module-two"))

			wareId, wareAddr, err := ws.GetCatalogWare(ref)
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, wareId.Hash, qt.Equals, "abcd")
			qt.Assert(t, wareId.Packtype, qt.Equals, wfapi.Packtype("tar"))
			qt.Assert(t, *wareAddr, qt.Equals, wfapi.WarehouseAddr("https://example.com/module/module-v1.0-x86_64.tgz"))
		})
		t.Run("catalog-replay", func(t *testing.T) {
			fsys := fstest.MapFS{
				"home/user/.warpforge/catalog/example.com/module/module.json": &fstest.MapFile{
					Mode: 0644,
					Data: []byte(moduleData),
				},
				"home/user/.warpforge/catalog/example.com/module/module.releases/v1.0.json": &fstest.MapFile{
					Mode: 0644,
					Data: []byte(releaseData),
				},
				"home/user/.warpforge/catalog/example.com/module/module.replays/zM5K3YuaVZAyaNzXnof5ixD5DW68WjuZYdgMduZJz75p9kxvgk8Mb3Hq8e6GMozwuSn7VF7.json": &fstest.MapFile{
					Mode: 0644,
					Data: []byte(replayData),
				},
				"home/user/.warpforge/catalog/example.com/module/mirrors.json": &fstest.MapFile{
					Mode: 0644,
					Data: []byte(mirrorData),
				},
			}
			var err error
			ws, _, err := FindWorkspace(fsys, "", "home/user/")
			cat, err := ws.OpenCatalog(nil)
			qt.Assert(t, err, qt.IsNil)
			_, err = cat.GetReplay(ref)
			qt.Assert(t, err, qt.IsNil)
		})

	})
}
