package workspace

import (
	"io/fs"
	"os"
	"testing"
	"testing/fstest"

	qt "github.com/frankban/quicktest"

	_ "github.com/warptools/warpforge/pkg/testutil"
)

// wssRootPaths returns just the paths from a list of workspaces; makes convenient diffables for test results.
func wssRootPaths(wss []*Workspace) []string {
	var res []string
	for _, ws := range wss {
		_, pth := ws.Path()
		res = append(res, pth)
	}
	return res
}

func TestHomeDir(t *testing.T) {
	hd, err := os.UserHomeDir()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, homedir, qt.Equals, hd[1:])
}

func TestFindWorkspaceStack(t *testing.T) {
	homedir = "home/user"

	t.Run("with home as default root workspace", func(t *testing.T) {
		t.Run("returns single+home workspace", func(t *testing.T) {
			fsys := fstest.MapFS{
				"home/user/foobar-proj/.warpforge": &fstest.MapFile{Mode: 0755 | fs.ModeDir},
				"home/user/.warphome":              &fstest.MapFile{Mode: 0755 | fs.ModeDir},
				"home/user/.warphome/root":         &fstest.MapFile{Mode: 0755},
			}
			wss, err := FindWorkspaceStack(fsys, "", "home/user/foobar-proj")
			qt.Check(t, err, qt.IsNil)
			qt.Check(t, wssRootPaths(wss), qt.DeepEquals, []string{
				"home/user/foobar-proj",
				"home/user",
			})
			qt.Check(t, wss[len(wss)-1].IsHomeWorkspace(), qt.IsTrue)
			qt.Check(t, wss[len(wss)-1].IsRootWorkspace(), qt.IsTrue)
		})
		t.Run("returns single+non-existent home workspace", func(t *testing.T) {
			fsys := fstest.MapFS{
				"home/user/foobar-proj/.warpforge": &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			}
			wss, err := FindWorkspaceStack(fsys, "", "home/user/foobar-proj")
			qt.Check(t, err, qt.IsNil)
			qt.Check(t, wssRootPaths(wss), qt.DeepEquals, []string{
				"home/user/foobar-proj",
				"home/user",
			})
			qt.Check(t, wss[len(wss)-1].IsHomeWorkspace(), qt.IsTrue)
			qt.Check(t, wss[len(wss)-1].IsRootWorkspace(), qt.IsTrue)
		})
		t.Run("returns nested workspaces", func(t *testing.T) {
			t.Run("from workspace dir", func(t *testing.T) {
				fsys := fstest.MapFS{
					"home/user/yaworkspace/.warpforge":           &fstest.MapFile{Mode: 0755 | fs.ModeDir},
					"home/user/yaworkspace/quux-proj/.warpforge": &fstest.MapFile{Mode: 0755 | fs.ModeDir},
				}
				wss, err := FindWorkspaceStack(fsys, "", "home/user/yaworkspace/quux-proj")
				qt.Check(t, err, qt.IsNil)
				qt.Check(t, wssRootPaths(wss), qt.DeepEquals, []string{
					"home/user/yaworkspace/quux-proj",
					"home/user/yaworkspace",
					"home/user",
				})
				qt.Check(t, wss[len(wss)-1].IsHomeWorkspace(), qt.IsTrue)

			})
			t.Run("from subdir", func(t *testing.T) {
				fsys := fstest.MapFS{
					"home/user/yaworkspace/.warpforge":           &fstest.MapFile{Mode: 0755 | fs.ModeDir},
					"home/user/yaworkspace/quux-proj/.warpforge": &fstest.MapFile{Mode: 0755 | fs.ModeDir},
					"home/user/yaworkspace/quux-proj/subdir":     &fstest.MapFile{Mode: 0755 | fs.ModeDir},
				}
				wss, err := FindWorkspaceStack(fsys, "", "home/user/yaworkspace/quux-proj/subdir")
				qt.Check(t, err, qt.IsNil)
				qt.Check(t, wssRootPaths(wss), qt.DeepEquals, []string{
					"home/user/yaworkspace/quux-proj",
					"home/user/yaworkspace",
					"home/user",
				})
				qt.Check(t, wss[len(wss)-1].IsHomeWorkspace(), qt.IsTrue)
			})
			t.Run("from non-existent subdir", func(t *testing.T) {
				fsys := fstest.MapFS{
					"home/user/yaworkspace/.warpforge":           &fstest.MapFile{Mode: 0755 | fs.ModeDir},
					"home/user/yaworkspace/quux-proj/.warpforge": &fstest.MapFile{Mode: 0755 | fs.ModeDir},
					"home/user/yaworkspace/quux-proj/subdir":     &fstest.MapFile{Mode: 0755 | fs.ModeDir},
				}
				wss, err := FindWorkspaceStack(fsys, "", "home/user/yaworkspace/quux-proj/subdir/not-a-real-path")
				qt.Check(t, err, qt.IsNil)
				qt.Check(t, wssRootPaths(wss), qt.DeepEquals, []string{
					"home/user/yaworkspace/quux-proj",
					"home/user/yaworkspace",
					"home/user",
				})
				qt.Check(t, wss[len(wss)-1].IsHomeWorkspace(), qt.IsTrue)
			})
			t.Run("ignores extra home workspace", func(t *testing.T) {
				fsys := fstest.MapFS{
					"home/user/yaworkspace/.warpforge":                   &fstest.MapFile{Mode: 0755 | fs.ModeDir},
					"home/user/yaworkspace/althome/.warphome":            &fstest.MapFile{Mode: 0755 | fs.ModeDir},
					"home/user/yaworkspace/althome/quux-proj/.warpforge": &fstest.MapFile{Mode: 0755 | fs.ModeDir},
				}
				wss, err := FindWorkspaceStack(fsys, "", "home/user/yaworkspace/althome/quux-proj")
				qt.Check(t, err, qt.IsNil)
				qt.Check(t, wssRootPaths(wss), qt.DeepEquals, []string{
					"home/user/yaworkspace/althome/quux-proj",
					"home/user/yaworkspace",
					"home/user",
				})
				qt.Check(t, wss[len(wss)-1].IsHomeWorkspace(), qt.IsTrue)
			})
		})
		t.Run("returns workspace at top-level directory", func(t *testing.T) {
			fsys := fstest.MapFS{
				".warpforge": &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			}
			wss, err := FindWorkspaceStack(fsys, "", "home/user/workspace/quux-proj/subdir/not-a-real-path")
			qt.Check(t, err, qt.IsNil)
			qt.Check(t, wssRootPaths(wss), qt.DeepEquals, []string{
				".",
				"home/user",
			})
			qt.Check(t, wss[len(wss)-1].IsHomeWorkspace(), qt.IsTrue)
		})
	})
	t.Run("with root workspace", func(t *testing.T) {
		fsys := fstest.MapFS{
			"home/user/yaworkspace/.warpforge":                                            &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"home/user/yaworkspace/althome/.warphome":                                     &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"home/user/yaworkspace/althome/root-workspace/workspace/.warpforge":           &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"home/user/yaworkspace/althome/root-workspace/workspace/quux-proj/.warpforge": &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"home/user/yaworkspace/althome/root-workspace/workspace/quux-proj/subdir":     &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"home/user/yaworkspace/althome/root-workspace/.warpforge/root":                &fstest.MapFile{Mode: 0755},
			"home/user/yaworkspace/althome/root-workspace/.warpforge":                     &fstest.MapFile{Mode: 0755 | fs.ModeDir},
		}
		t.Run("returns nested workspaces", func(t *testing.T) {
			wss, err := FindWorkspaceStack(fsys, "", "home/user/yaworkspace/althome/root-workspace/workspace/quux-proj")
			qt.Check(t, err, qt.IsNil)
			qt.Check(t, wssRootPaths(wss), qt.DeepEquals, []string{
				"home/user/yaworkspace/althome/root-workspace/workspace/quux-proj",
				"home/user/yaworkspace/althome/root-workspace/workspace",
				"home/user/yaworkspace/althome/root-workspace",
			})
			qt.Check(t, wss[len(wss)-1].IsHomeWorkspace(), qt.IsFalse)
			qt.Check(t, wss[len(wss)-1].IsRootWorkspace(), qt.IsTrue)
		})
		t.Run("returns nested workspaces from subdir", func(t *testing.T) {
			wss, err := FindWorkspaceStack(fsys, "", "home/user/yaworkspace/althome/root-workspace/workspace/quux-proj/subdir")
			qt.Check(t, err, qt.IsNil)
			qt.Check(t, wssRootPaths(wss), qt.DeepEquals, []string{
				"home/user/yaworkspace/althome/root-workspace/workspace/quux-proj",
				"home/user/yaworkspace/althome/root-workspace/workspace",
				"home/user/yaworkspace/althome/root-workspace",
			})
			qt.Check(t, wss[len(wss)-1].IsHomeWorkspace(), qt.IsFalse)
			qt.Check(t, wss[len(wss)-1].IsRootWorkspace(), qt.IsTrue)
		})
	})
	t.Run("with basis path", func(t *testing.T) {
		fsys := fstest.MapFS{
			"home/user/.warphome/root":                                                    &fstest.MapFile{Mode: 0755},
			"home/user/.warphome":                                                         &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"home/user/yaworkspace/.warpforge":                                            &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"home/user/yaworkspace/althome/.warphome/root":                                &fstest.MapFile{Mode: 0755},
			"home/user/yaworkspace/althome/.warphome":                                     &fstest.MapFile{Mode: 0755 | fs.ModeDir}, // althome exists as a test that we don't detect extra .warphome directories
			"home/user/yaworkspace/althome/root-workspace/.warpforge/root":                &fstest.MapFile{Mode: 0755},
			"home/user/yaworkspace/althome/root-workspace/.warpforge":                     &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"home/user/yaworkspace/althome/root-workspace/workspace/.warpforge":           &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"home/user/yaworkspace/althome/root-workspace/workspace/quux-proj/.warpforge": &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"home/user/yaworkspace/althome/root-workspace/workspace/quux-proj/subdir":     &fstest.MapFile{Mode: 0755 | fs.ModeDir},
		}
		t.Run("with no search path, returns basis path if it is a workspace", func(t *testing.T) {
			wss, err := FindWorkspaceStack(fsys, "home/user/yaworkspace/althome/root-workspace/workspace/quux-proj", "")
			qt.Check(t, err, qt.IsNil)
			qt.Check(t, wss, qt.IsNotNil)
			qt.Check(t, wssRootPaths(wss), qt.DeepEquals, []string{
				"home/user/yaworkspace/althome/root-workspace/workspace/quux-proj",
				"home/user",
			})
			qt.Check(t, wss[len(wss)-1].IsHomeWorkspace(), qt.IsTrue)
			qt.Check(t, wss[len(wss)-1].IsRootWorkspace(), qt.IsTrue)
		})
		t.Run("returns multiple workspaces up to and including basis path", func(t *testing.T) {
			wss, err := FindWorkspaceStack(fsys, "home/user/yaworkspace/althome/root-workspace/workspace", "quux-proj/subdir")
			qt.Check(t, err, qt.IsNil)
			qt.Check(t, wss, qt.IsNotNil)
			qt.Check(t, wssRootPaths(wss), qt.DeepEquals, []string{
				"home/user/yaworkspace/althome/root-workspace/workspace/quux-proj",
				"home/user/yaworkspace/althome/root-workspace/workspace",
				"home/user",
			})
			qt.Check(t, wss[len(wss)-1].IsHomeWorkspace(), qt.IsTrue)
			qt.Check(t, wss[len(wss)-1].IsRootWorkspace(), qt.IsTrue)
		})
		t.Run("returns up to first root workspace", func(t *testing.T) {
			wss, err := FindWorkspaceStack(fsys, "home/user/yaworkspace", "althome/root-workspace/workspace/quux-proj/subdir")
			qt.Check(t, err, qt.IsNil)
			qt.Check(t, wss, qt.IsNotNil)
			qt.Check(t, wssRootPaths(wss), qt.DeepEquals, []string{
				"home/user/yaworkspace/althome/root-workspace/workspace/quux-proj",
				"home/user/yaworkspace/althome/root-workspace/workspace",
				"home/user/yaworkspace/althome/root-workspace",
			})
			qt.Check(t, wss[len(wss)-1].IsHomeWorkspace(), qt.IsFalse)
			qt.Check(t, wss[len(wss)-1].IsRootWorkspace(), qt.IsTrue)
		})
	})
}

func TestFindWorkspace(t *testing.T) {
	homedir = "home/user"
	t.Run("returns workspace", func(t *testing.T) {
		fsys := fstest.MapFS{
			"workspace/.warpforge":             &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"workspace/foobar-proj":            &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"workspace/foobar-proj/.warpforge": &fstest.MapFile{Mode: 0755 | fs.ModeDir},
		}
		ws, rem, err := FindWorkspace(fsys, "", "workspace/foobar-proj")
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, ws, qt.IsNotNil)
		qt.Check(t, ws.rootPath, qt.Equals, "workspace/foobar-proj")
		qt.Check(t, rem, qt.Equals, "workspace")
	})
	t.Run("returns workspace from subdir", func(t *testing.T) {
		fsys := fstest.MapFS{
			"path/workspace/.warpforge": &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"path/workspace/subdir":     &fstest.MapFile{Mode: 0755 | fs.ModeDir},
		}
		ws, rem, err := FindWorkspace(fsys, "", "path/workspace/subdir/non-existent-subdir")
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, ws, qt.IsNotNil)
		qt.Check(t, ws.rootPath, qt.Equals, "path/workspace")
		qt.Check(t, rem, qt.Equals, "path")
	})
	t.Run("does not return extra home workspace", func(t *testing.T) {
		fsys := fstest.MapFS{
			"home/user/.warphome":         &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"workspace/.warpforge":        &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"workspace/nothome/.warphome": &fstest.MapFile{Mode: 0755 | fs.ModeDir},
		}
		{
			ws, rem, err := FindWorkspace(fsys, "", "home/user")
			qt.Check(t, err, qt.IsNil)
			qt.Check(t, ws, qt.IsNil)
			qt.Check(t, rem, qt.Equals, "")
		}
		{
			ws, rem, err := FindWorkspace(fsys, "", "workspace/nothome")
			qt.Check(t, err, qt.IsNil)
			qt.Assert(t, ws, qt.IsNotNil)
			qt.Check(t, ws.rootPath, qt.Equals, "workspace")
			qt.Check(t, rem, qt.Equals, "")
		}
	})
	t.Run("returns nothing when workspace internals is a regular file", func(t *testing.T) {
		fsys := fstest.MapFS{
			"home/user/fake":            &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"home/user/fake/.warpforge": &fstest.MapFile{Mode: 0755}, // Explicitly NOT fs.ModeDir
		}
		_, statErr := fs.Stat(fsys, "home/user/fake/.warpforge")
		qt.Check(t, statErr, qt.IsNil)
		ws, rem, err := FindWorkspace(fsys, "", "home/user/fake")
		qt.Check(t, err, qt.IsNil)
		qt.Check(t, ws, qt.IsNil)
		qt.Check(t, rem, qt.Equals, "")
	})
	t.Run("returns workspace from top level directory", func(t *testing.T) {
		fsys := fstest.MapFS{
			".warpforge": &fstest.MapFile{Mode: 0755 | fs.ModeDir},
		}
		ws, rem, err := FindWorkspace(fsys, "", "a/path/to/nowhere")
		qt.Check(t, err, qt.IsNil)
		qt.Assert(t, ws, qt.IsNotNil)
		qt.Check(t, rem, qt.Equals, "")
		qt.Check(t, ws.rootPath, qt.Equals, ".")
		qt.Check(t, ws.IsHomeWorkspace(), qt.IsFalse)
		qt.Check(t, ws.IsRootWorkspace(), qt.IsFalse)
	})
	t.Run("return workspace when basis path is workspace path", func(t *testing.T) {
		fsys := fstest.MapFS{
			"workspace/.warpforge": &fstest.MapFile{Mode: 0755 | fs.ModeDir},
		}
		ws, rem, err := FindWorkspace(fsys, "workspace", "a/path/to/nowhere")
		qt.Check(t, err, qt.IsNil)
		qt.Assert(t, ws, qt.IsNotNil)
		qt.Check(t, rem, qt.Equals, "")
		qt.Check(t, ws.rootPath, qt.Equals, "workspace")
		qt.Check(t, ws.IsHomeWorkspace(), qt.IsFalse)
		qt.Check(t, ws.IsRootWorkspace(), qt.IsFalse)
	})
	t.Run("return workspace when search path repeats basis path ", func(t *testing.T) {
		fsys := fstest.MapFS{
			"workspace/.warpforge": &fstest.MapFile{Mode: 0755 | fs.ModeDir},
		}
		ws, rem, err := FindWorkspace(fsys, "workspace", "workspace/a/path/to/nowhere")
		qt.Check(t, err, qt.IsNil)
		qt.Assert(t, ws, qt.IsNotNil)
		qt.Check(t, rem, qt.Equals, "")
		qt.Check(t, ws.rootPath, qt.Equals, "workspace")
		qt.Check(t, ws.IsHomeWorkspace(), qt.IsFalse)
		qt.Check(t, ws.IsRootWorkspace(), qt.IsFalse)
	})
}
