package wfapp_test

import (
	"testing"

	"github.com/warptools/warpforge/app/testutil"
)

func TestExampleDirCLI(t *testing.T) {
	testutil.TestFileContainingTestmarkexec(t, "../examples/500-cli/cli.md", nil)
}

func TestExampleDirCLIHelp(t *testing.T) {
	testutil.TestFileContainingTestmarkexec(t, "../examples/500-cli/help.md", nil)
}
