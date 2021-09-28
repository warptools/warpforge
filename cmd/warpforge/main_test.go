package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/warpfork/go-testmark"
	"github.com/warpfork/go-testmark/testexec"
)

func TestFormulaExecFixtures(t *testing.T) {
	os.Chdir("../../examples/500-cli")
	doc, err := testmark.ReadFile("cli.md")
	if err != nil {
		t.Fatalf("spec file parse failed?!: %s", err)
	}

	// Data hunk in this spec file are in "directories" of a test scenario each.
	doc.BuildDirIndex()
	for _, dir := range doc.DirEnt.ChildrenList {
		t.Run(dir.Name, func(t *testing.T) {
			switch {
			case dir.Children["cmd"] != nil:
				// Nab the bytes.
				command := strings.TrimSpace(string(dir.Children["cmd"].Hunk.Body))

				t.Run("exec-cli", func(t *testing.T) {
					// TODO capture stdout/stderr
					exitCode, err := Run(strings.Split(command, " "), os.Stdin, os.Stdout, os.Stderr)
					qt.Assert(t, exitCode, qt.Equals, 0)
					qt.Assert(t, err, qt.IsNil)
				})
			}
		})
	}
}

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
			ExecFn:   Run,
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

func assertFn(t *testing.T, actual, expect string) {
	actual = cleanRunRecord(actual)
	expect = cleanRunRecord(expect)
	qt.Assert(t, actual, qt.Equals, expect)
}
