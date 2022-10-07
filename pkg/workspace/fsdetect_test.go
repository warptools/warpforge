package workspace

import (
	"io/fs"
	"testing"
	"testing/fstest"

	qt "github.com/frankban/quicktest"
	_ "github.com/warpfork/warpforge/pkg/testutil"
)

// projects out just the paths from a list of workspaces; makes convenient diffables for test results.
func projectWorkspacePaths(wss []*Workspace) []string {
	var res []string
	for _, ws := range wss {
		_, pth := ws.Path()
		res = append(res, pth)
	}
	return res
}

func TestWorkspaceDetection(t *testing.T) {
	homedir = "home/user"
	t.Run("happy-path", func(t *testing.T) {
		fsys := fstest.MapFS{
			"home/user/.warpforge":                     &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"home/user/foobar-proj/.warpforge":         &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"home/user/workspace/.warpforge":           &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"home/user/workspace/quux-proj/.warpforge": &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"home/user/workspace/quux-proj/subdir":     &fstest.MapFile{Mode: 0755 | fs.ModeDir},
		}
		// fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		// 	fmt.Printf(":fs: %s -- %#v\n", path, d)
		// 	return nil
		// })
		t.Run("find-returns-workspace", func(t *testing.T) {
			ws, _, err := FindWorkspace(fsys, "", "home/user/foobar-proj")
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, ws, qt.IsNotNil)
			qt.Check(t, ws.rootPath, qt.Equals, "home/user/foobar-proj")
		})
		t.Run("find-returns-workspace-from-subdir", func(t *testing.T) {
			ws, _, err := FindWorkspace(fsys, "", "home/user/workspace/quux-proj/subdir")
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, ws, qt.IsNotNil)
			qt.Check(t, ws.rootPath, qt.Equals, "home/user/workspace/quux-proj")
		})
		t.Run("find-does-not-return-home-workspace", func(t *testing.T) {
			ws, _, err := FindWorkspace(fsys, "", "home/user")
			qt.Check(t, err, qt.IsNil)
			qt.Check(t, ws, qt.IsNil)
		})
		t.Run("find-stack-works-in-project", func(t *testing.T) {
			wss, err := FindWorkspaceStack(fsys, "", "home/user/foobar-proj")
			qt.Check(t, err, qt.IsNil)
			qt.Check(t, projectWorkspacePaths(wss), qt.DeepEquals, []string{
				"home/user/foobar-proj",
				"home/user",
			})
			qt.Check(t, wss[len(wss)-1].IsHomeWorkspace(), qt.IsTrue)
			qt.Check(t, wss[len(wss)-1].IsRootWorkspace(), qt.IsTrue)
		})
		t.Run("find-stack-works-in-nested-project", func(t *testing.T) {
			wss, err := FindWorkspaceStack(fsys, "", "home/user/workspace/quux-proj")
			qt.Check(t, err, qt.IsNil)
			qt.Check(t, projectWorkspacePaths(wss), qt.DeepEquals, []string{
				"home/user/workspace/quux-proj",
				"home/user/workspace",
				"home/user",
			})
			qt.Check(t, wss[len(wss)-1].IsHomeWorkspace(), qt.IsTrue)
		})
		t.Run("find-stack-works-in-nested-project-subdir", func(t *testing.T) {
			wss, err := FindWorkspaceStack(fsys, "", "home/user/workspace/quux-proj/subdir")
			qt.Check(t, err, qt.IsNil)
			qt.Check(t, projectWorkspacePaths(wss), qt.DeepEquals, []string{
				"home/user/workspace/quux-proj",
				"home/user/workspace",
				"home/user",
			})
			qt.Check(t, wss[len(wss)-1].IsHomeWorkspace(), qt.IsTrue)
		})
	})
	t.Run("happy-path-with-root-workspace", func(t *testing.T) {
		fsys := fstest.MapFS{
			"home/user/.warpforge":                                               &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"home/user/yaworkspace/.warpforge":                                   &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"home/user/yaworkspace/rootworkspace/.warpforge":                     &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"home/user/yaworkspace/rootworkspace/.warpforge/root":                &fstest.MapFile{Mode: 0755},
			"home/user/yaworkspace/rootworkspace/workspace/.warpforge":           &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"home/user/yaworkspace/rootworkspace/workspace/quux-proj/.warpforge": &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"home/user/yaworkspace/rootworkspace/workspace/quux-proj/subdir":     &fstest.MapFile{Mode: 0755 | fs.ModeDir},
		}
		// fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		// 	fmt.Printf(":fs: %s -- %#v\n", path, d)
		// 	return nil
		// })
		t.Run("find-returns-root-workspace", func(t *testing.T) {
			ws, _, err := FindWorkspace(fsys, "", "home/user/yaworkspace/rootworkspace")
			qt.Check(t, err, qt.IsNil)
			qt.Assert(t, ws, qt.IsNotNil)
			qt.Check(t, ws.rootPath, qt.Equals, "home/user/yaworkspace/rootworkspace")
			qt.Check(t, ws.IsHomeWorkspace(), qt.IsFalse)
			qt.Check(t, ws.IsRootWorkspace(), qt.IsTrue)
		})
		t.Run("find-stack-works-in-nested-project", func(t *testing.T) {
			wss, err := FindWorkspaceStack(fsys, "", "home/user/yaworkspace/rootworkspace/workspace/quux-proj")
			qt.Check(t, err, qt.IsNil)
			qt.Check(t, projectWorkspacePaths(wss), qt.DeepEquals, []string{
				"home/user/yaworkspace/rootworkspace/workspace/quux-proj",
				"home/user/yaworkspace/rootworkspace/workspace",
				"home/user/yaworkspace/rootworkspace",
			})
			qt.Check(t, wss[len(wss)-1].IsHomeWorkspace(), qt.IsFalse)
			qt.Check(t, wss[len(wss)-1].IsRootWorkspace(), qt.IsTrue)
		})
		t.Run("find-stack-works-in-nested-project-subdir", func(t *testing.T) {
			wss, err := FindWorkspaceStack(fsys, "", "home/user/yaworkspace/rootworkspace/workspace/quux-proj/subdir")
			qt.Check(t, err, qt.IsNil)
			qt.Check(t, projectWorkspacePaths(wss), qt.DeepEquals, []string{
				"home/user/yaworkspace/rootworkspace/workspace/quux-proj",
				"home/user/yaworkspace/rootworkspace/workspace",
				"home/user/yaworkspace/rootworkspace",
			})
			qt.Check(t, wss[len(wss)-1].IsHomeWorkspace(), qt.IsFalse)
			qt.Check(t, wss[len(wss)-1].IsRootWorkspace(), qt.IsTrue)
		})
	})
	t.Run("unhappy-path-workspace-not-a-dir", func(t *testing.T) {
		fsys := fstest.MapFS{
			"home/user/foo":            &fstest.MapFile{Mode: 0755 | fs.ModeDir},
			"home/user/foo/.warpforge": &fstest.MapFile{Mode: 0755},
		}
		_, statErr := fs.Stat(fsys, "home/user/foo/.warpforge")
		qt.Check(t, statErr, qt.IsNil)
		ws, _, err := FindWorkspace(fsys, "", "home/user/foo")
		qt.Check(t, err, qt.IsNil)
		qt.Check(t, ws, qt.IsNil)
	})
}
