package workspace

import (
	"io/fs"
	"testing"
	"testing/fstest"

	qt "github.com/frankban/quicktest"
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
		t.Run("home-workspace-not-returned-by-find", func(t *testing.T) {
		})
		t.Run("find-stack-works-in-project", func(t *testing.T) {
			wss, err := FindWorkspaceStack(fsys, "", "home/user/foobar-proj")
			qt.Check(t, err, qt.IsNil)
			qt.Check(t, projectWorkspacePaths(wss), qt.DeepEquals, []string{
				"home/user/foobar-proj",
				"home/user",
			})
			qt.Check(t, wss[len(wss)-1].IsHomeWorkspace(), qt.IsTrue)
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
}
