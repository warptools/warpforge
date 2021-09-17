package main

import (
	"os"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/warpfork/go-testmark"
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
					err := Run(strings.Split(command, " "), os.Stdin, os.Stdout, os.Stderr)
					qt.Assert(t, err, qt.IsNil)
				})
			}
		})
	}
}
