package wfapp_test

import (
	"testing"

	"github.com/warptools/warpforge/app/testutil"
)

/*
	This doesn't include *all* tests that target content in the examples dir...
	just the ones that also have whole-system coverage.
*/

func TestExampleDirCLI(t *testing.T) {
	testutil.TestFileContainingTestmarkexec(t, "../examples/500-cli/cli.md", nil)
}

func TestExampleDirCLIHelp(t *testing.T) {
	testutil.TestFileContainingTestmarkexec(t, "../examples/500-cli/help.md", nil)
}
