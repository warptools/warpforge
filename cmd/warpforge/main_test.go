package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/warpfork/go-testmark"
	"github.com/warpfork/go-testmark/testexec"
	"github.com/warpfork/warpforge/pkg/workspace"
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
		t.Run(dir.Name, func(t *testing.T) {
			test := testexec.Tester{
				ExecFn:   execFn,
				Patches:  &patches,
				AssertFn: assertFn,
			}
			test.Test(t, dir)
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
		testmarkWd, err := os.Getwd()
		if err != nil {
			return 1, fmt.Errorf("failed to get testmark pwd: %s", err)
		}

		// override the path to required binaries
		err = os.Setenv("WARPFORGE_PATH", filepath.Join(projPath, "plugins"))

		os.MkdirAll(filepath.Join(testmarkWd, ".warpforge/catalogs/default"), 0755)
		copyCmd := exec.Command("cp", "--recursive", filepath.Join(projPath, ".warpforge", "catalog"), filepath.Join(testmarkWd, ".warpforge", "catalogs"))
		copyCmd.Run()
		copyCmd = exec.Command("cp", "--recursive", filepath.Join(projPath, ".warpforge", "warehouse"), filepath.Join(testmarkWd, ".warpforge"))
		copyCmd.Run()
		os.Create(filepath.Join(testmarkWd, ".warpforge", "root"))

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
