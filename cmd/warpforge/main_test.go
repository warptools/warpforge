package main

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/warpfork/go-testmark"
	"github.com/warpfork/go-testmark/testexec"

	"github.com/warptools/warpforge/pkg/config"
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

type tagset map[string]struct{}

func newTagSet(tags ...string) tagset {
	result := tagset(make(map[string]struct{}))
	for _, s := range tags {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		result[s] = struct{}{}
	}
	return result
}

func (t tagset) has(tag string) bool {
	if t == nil {
		return false
	}
	_, ok := t[tag]
	return ok
}

func testFile(t *testing.T, fileName string, workDir *string) {
	doc, err := testmark.ReadFile(fileName)
	if err != nil {
		t.Fatalf("spec file parse failed?!: %s", err)
	}

	pwd, err := os.Getwd()
	qt.Assert(t, err, qt.IsNil)

	if workDir != nil {
		err = os.Chdir(*workDir)
		qt.Assert(t, err, qt.IsNil)
	}

	doc.BuildDirIndex()
	patches := testmark.PatchAccumulator{}
	defer func() {
		if *testmark.Regen {
			patches.WriteFileWithPatches(doc, fileName)
		}
	}()
	for _, dir := range doc.DirEnt.ChildrenList {
		testName := dir.Name
		testDir := dir
		tags := getTags(testDir)
		if tags != nil {
			if len(testDir.Children) != 1 {
				t.Run(testName, func(t *testing.T) {
					t.Fatal("tagged tests must place children after the /tag=.../ dir")
				})
				continue
			}
			testDir = testDir.ChildrenList[0]
			testName = testName + "/" + testDir.Name
		}
		t.Run(testName, func(t *testing.T) {
			if tags.has("net") && *testutil.FlagOffline {
				t.Skip("skipping test", t.Name(), "due to offline flag")
			}
			// build an exec function with a pointer to this project's git root
			execFn := buildExecFn(t, filepath.Join(pwd, "../../"))
			qt.Assert(t, err, qt.IsNil)

			test := testexec.Tester{
				ExecFn:   execFn,
				Patches:  &patches,
				AssertFn: assertFn,
			}
			test.Test(t, testDir)
		})
	}
	if *testmark.Regen {
		patches.WriteFileWithPatches(doc, fileName)
	}
}

// getTags will return the tagset for the first child it finds with the prefix `tags=`
// The tags following the prefix are expected to be comma separated strings.
func getTags(dir *testmark.DirEnt) tagset {
	for _, child := range dir.ChildrenList {
		if strings.HasPrefix(child.Name, "tags=") {
			return newTagSet(strings.Split(child.Name[len("tags="):], ",")...)
		}
	}
	return nil
}

func TestExecFixtures(t *testing.T) {
	file := "../../examples/500-cli/cli.md"
	t.Logf("loading test file: %q", file)
	testFile(t, file, nil)
}

func TestHelpFixtures(t *testing.T) {
	file := "../../examples/500-cli/help.md"
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

func buildExecFn(t *testing.T, projPath string) func(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) (int, error) {
	return func(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		// override the path to required binaries
		pluginPath := filepath.Join(projPath, "plugins")
		err := os.Setenv("WARPFORGE_PATH", pluginPath)
		if err != nil {
			panic("failed to set WARPFORGE_PATH")
		}
		// use this project's warehouse as the warehouse for tests
		err = os.Setenv("WARPFORGE_WAREHOUSE", filepath.Join(projPath, ".warpforge", "warehouse"))
		if err != nil {
			panic("failed to set WARPFORGE_WAREHOUSE")
		}
		if args[0] == "cd" {
			if err := os.Chdir(args[1]); err != nil {
				return 1, err
			}
			return 0, nil
		}
		if err := config.ReloadGlobalState(); err != nil {
			// This will reset values loaded from environment variables
			panic("failed to reset global state")
		}
		err = makeApp(stdin, stdout, stderr).Run(args)
		if err != nil {
			t.Logf("Exec Error: %s", err)
			return 1, nil
		}
		return 0, nil
	}
}

func assertFn(t *testing.T, actual, expect string) {
	actual = cleanRunRecord(actual)
	expect = cleanRunRecord(expect)
	qt.Assert(t, actual, qt.Equals, expect)
}
