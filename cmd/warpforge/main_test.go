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
		if _, hasNetTests := dir.Children["net"]; hasNetTests {
			if *testutil.FlagOffline {
				t.Log("skipping test", dir.Name, "due to offline flag")
				continue
			}
			// we want to run the contents of the `/net` dir
			testName = dir.Name + "/net"
			testDir = dir.Children["net"]
		}
		t.Run(testName, func(t *testing.T) {
			test := testexec.Tester{
				ExecFn:   execFn,
				Patches:  &patches,
				AssertFn: assertFn,
			}
			test.Test(t, testDir)
		})
	}
}

func TestExecFixtures(t *testing.T) {
	testFile(t, "../../examples/500-cli/cli.md", nil)
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
