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

func TestExecFixtures(t *testing.T) {
	os.Chdir("../../examples/500-cli")
	doc, err := testmark.ReadFile("cli.md")
	if err != nil {
		t.Fatalf("spec file parse failed?!: %s", err)
	}

	// override the path to required binaries
	pwd, err := os.Getwd()
	qt.Assert(t, err, qt.IsNil)
	err = os.Setenv("WARPFORGE_PATH", filepath.Join(pwd, "../../plugins"))
	qt.Assert(t, err, qt.IsNil)
	err = os.Setenv("WARPFORGE_HOME", filepath.Join(pwd, "../../.test-home"))
	qt.Assert(t, err, qt.IsNil)

	doc.BuildDirIndex()
	patches := testmark.PatchAccumulator{}
	for _, dir := range doc.DirEnt.ChildrenList {
		test := testexec.Tester{
			ExecFn:   execFn,
			Patches:  &patches,
			AssertFn: assertFn,
		}
		test.TestSequence(t, dir)
	}
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

func execFn(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	err := makeApp(stdin, stdout, stderr).Run(args)
	if err != nil {
		return 1, err
	}
	return 0, nil
}

func assertFn(t *testing.T, actual, expect string) {
	actual = cleanRunRecord(actual)
	expect = cleanRunRecord(expect)
	qt.Assert(t, actual, qt.Equals, expect)
}
