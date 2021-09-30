package main

import (
	"io"
	"os"
	"regexp"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/warpfork/go-testmark"
	"github.com/warpfork/go-testmark/testexec"
)

func TestExecFixtures(t *testing.T) {
	os.Chdir("../../examples/500-cli")
	doc, err := testmark.ReadFile("cli.md")
	if err != nil {
		t.Fatalf("spec file parse failed?!: %s", err)
	}

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
