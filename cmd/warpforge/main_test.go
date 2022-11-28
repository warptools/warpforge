package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/warpfork/go-testmark"
	"github.com/warpfork/go-testmark/testexec"

	"github.com/warptools/warpforge/pkg/testutil"
	"github.com/warptools/warpforge/pkg/workspace"
)

// constructs a custom workspace stack containing only this project's .warpforge dir (contains catalog)
func getTestWorkspaceStack(t *testing.T) []*workspace.Workspace {
	pwd, err := os.Getwd()
	qt.Assert(t, err, qt.IsNil)
	projWs, err := workspace.OpenWorkspace(os.DirFS("/"), filepath.Join(pwd[1:], "../../"))
	qt.Assert(t, err, qt.IsNil)
	wss := []*workspace.Workspace{
		projWs,
	}
	return wss
}

func testFile(t *testing.T, fileName string, workDir *string) {
	doc, err := testmark.ReadFile(fileName)
	if err != nil {
		t.Fatalf("spec file parse failed?!: %s", err)
	}

	pwd, err := os.Getwd()
	qt.Assert(t, err, qt.IsNil)

	// build an exec function with a pointer to this project's git root
	execFn := buildExecFn(filepath.Join(pwd, "../../"))
	qt.Assert(t, err, qt.IsNil)

	if workDir != nil {
		err = os.Chdir(*workDir)
		qt.Assert(t, err, qt.IsNil)
	}

	doc.BuildDirIndex()
	patches := testmark.PatchAccumulator{}
	for _, dir := range doc.DirEnt.ChildrenList {
		testName := dir.Name
		testDir := dir
		if testDir, hasNetTests := dir.Children["net"]; hasNetTests {
			// copy the non "then-*" children to the "net" dir
			// this duplicates some work but ensures that any prerequisite
			// commands are executed and files are placed.
			// Implicitly this means that a "net" dir and its parent cannot
			// both have executable statements and/or "fs" nodes.
			if err := inheritChildren(dir, "net"); err != nil {
				t.Fatal(err)
			}
			t.Run(testName+"/net", func(t *testing.T) {
				if *testutil.FlagOffline {
					t.Skip("skipping test", t.Name(), "due to offline flag")
				}
				test := testexec.Tester{
					ExecFn:   execFn,
					Patches:  &patches,
					AssertFn: assertFn,
				}
				test.Test(t, testDir)
			})
			removeChildByName(dir, "net")
			if len(dir.Children) == 0 {
				continue
			}
		}
		// non-net tests
		t.Run(testName, func(t *testing.T) {
			if strings.HasSuffix(t.Name(), "net") && *testutil.FlagOffline {
				t.Skip("skipping test", t.Name(), "due to offline flag")
			}
			test := testexec.Tester{
				ExecFn:   execFn,
				Patches:  &patches,
				AssertFn: assertFn,
			}
			test.Test(t, testDir)
		})
	}
}

func names(dirs []*testmark.DirEnt) []string {
	result := make([]string, 0, len(dirs))
	for _, d := range dirs {
		result = append(result, d.Name)
	}
	return result
}

// inheritChildren will copy pointers to dir's children to the named child.
// Copying excludes the named child itself and any child beginning with "then-"
func inheritChildren(dir *testmark.DirEnt, childName string) error {
	target := dir.Children[childName]
	inherited := []*testmark.DirEnt{}
	for _, child := range dir.ChildrenList {
		if child.Name == childName {
			continue
		}
		if strings.HasPrefix(child.Name, "then-") {
			continue
		}
		if _, exists := target.Children[child.Name]; exists {
			return fmt.Errorf("cannot copy child %q from %q to %q: %w", child.Name, dir.Name, target.Name, os.ErrExist)
		}
		target.Children[child.Name] = child
		inherited = append(inherited, child)
	}
	target.ChildrenList = append(inherited, target.ChildrenList...)
	return nil
}

// removeChildByName removes references to the named child
// from the given testmark.DirEnt
func removeChildByName(dir *testmark.DirEnt, name string) {
	n := 0
	for _, child := range dir.ChildrenList {
		if child.Name != name {
			dir.ChildrenList[n] = child
			n++
		}
	}
	dir.ChildrenList = dir.ChildrenList[:n]
	delete(dir.Children, name)
	if len(dir.ChildrenList) != len(dir.Children) {
		panic(`we messed up: removing child should not make list/map of children have different lengths`)
	}
}

func TestExecFixtures(t *testing.T) {
	file := "../../examples/500-cli/cli.md"
	t.Logf("loading test file: %q", file)
	testFile(t, file, nil)
}

// Replace non-deterministic values of JSON runrecord to allow for deterministic comparison
func cleanRunRecord(str string) string {
	// replace guid
	matcher := regexp.MustCompile(`"guid": "[a-zA-Z0-9]{8}-[a-zA-Z0-9]{4}-[a-zA-Z0-9]{4}-[a-zA-Z0-9]{4}-[a-zA-Z0-9]{12}"`)
	str = matcher.ReplaceAllString(str, `"guid": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"`)

	// replace time
	matcher = regexp.MustCompile(`"time": [0-9]+`)
	str = matcher.ReplaceAllString(str, `"time": "22222222222"`)

	// return value with whitespace trimmed
	return strings.TrimSpace(str)
}

func buildExecFn(projPath string) func(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) (int, error) {
	return func(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		// override the path to required binaries
		err := os.Setenv("WARPFORGE_PATH", filepath.Join(projPath, "plugins"))
		if err != nil {
			panic("failed to set WARPFORGE_PATH")
		}
		// use this project's warehouse as the warehouse for tests
		err = os.Setenv("WARPFORGE_WAREHOUSE", filepath.Join(projPath, ".warpforge", "warehouse"))
		if err != nil {
			panic("failed to set WARPFORGE_WAREHOUSE")
		}

		err = makeApp(stdin, stdout, stderr).Run(args)
		if err != nil {
			return 1, err
		}
		return 0, nil
	}
}

func assertFn(t *testing.T, actual, expect string) {
	actual = cleanRunRecord(actual)
	expect = cleanRunRecord(expect)
	qt.Assert(t, actual, qt.Equals, expect)
}
