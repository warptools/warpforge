package wfapp_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/frankban/quicktest"
	"github.com/urfave/cli/v2"
	"github.com/warpfork/go-fsx/osfs"
	"github.com/warpfork/go-testmark"
	"github.com/warpfork/go-testmark/suite"

	wfapp "github.com/warptools/warpforge/app"
	"github.com/warptools/warpforge/app/base/helpgen"
	"github.com/warptools/warpforge/app/base/render"
)

const fixtureDir = "_docs"

// TestCommandDocs tests all the commands and their docs to match.
// This requires the warpforge-site repo to be available.
//
// This uses testmark.SuiteManager to glob a file per each subcommand.
// Another test is responsible for making sure a file exists per subcommand.
// Between the two of them, this effectively makes sure there's exactly one
// file per subcommand, and any stale files get trimmed (because they'd cause
// a test to be created which would flunk if that command doesn't exist).
//
func TestCommandDocs(t *testing.T) {
	suite := suite.NewManager(osfs.DirFS(fixtureDir))
	suite.MustWorkWith("warpforge*.md", "docs", testingPattern{})
	suite.DisableFileParallelism()
	suite.Run(t)
}

type testingPattern struct{}

func (tp testingPattern) Name() string          { return "cli doc test" }
func (tp testingPattern) OwnsAllChildren() bool { return false }
func (tp testingPattern) Run(
	t *testing.T,
	filename string,
	subject *testmark.DirEnt,
	reportUse func(string),
	reportUnrecog func(string, string),
	patchAccum *testmark.PatchAccumulator,
) error {
	reportUse(subject.Path)
	command := strings.Split(strings.Split(filename, ".md")[0], "-")
	var buf bytes.Buffer
	wfapp.App.Writer = &buf
	wfapp.App.ErrWriter = &buf
	helpgen.Mode = render.Mode_Markdown
	wfapp.App.Run(append(command, "-h"))
	if patchAccum != nil {
		newHunk := *subject.Hunk
		newHunk.Body = buf.Bytes()
		patchAccum.AppendPatch(newHunk)
		return nil
	}
	quicktest.Assert(t, buf.String(), quicktest.Equals, string(subject.Hunk.Body))
	return nil
}

// TestAllCommandsHaveDocFile just walks through the CLI and checks that
// a markdown file exists for every single command.
// (It doesn't test that those files contain docs hunks, but the testmark suites setup does.)
func TestAllCommandsHaveDocFile(t *testing.T) {
	t.Skip("this isn't even close to passing right now :)")

	commandNames := collectAllCommandNames(wfapp.App)
	afs := os.DirFS(fixtureDir)
	for _, cmdName := range commandNames {
		fileName := cmdNameToFilename(cmdName)
		_, err := fs.Stat(afs, fileName)
		if errors.Is(err, fs.ErrNotExist) {
			if *testmark.Regen {
				// FUTURE: write stub file?  (not sure if useful.)
			} else {
				t.Errorf("expected a file named %q for documenting the `%s` command", fileName, cmdName)
			}
		}
		// No need to peek that it has the minimum the expected hunk, too, because our SuiteManager setup effectively does that.
	}
}

func cmdNameToFilename(cmdName string) string {
	return strings.Replace(cmdName, " ", "-", -1) + ".md"
}

// TestNoExcessCommandDocFiles walks through the docs directories
// and makes sure no file exists that doesn't have a matching subcommand.
// This helps ensure we don't have stale docs that refer to any removed or renamed commands.
//
// JK, there's no need for this since we run everything based on a glob,
// and if the thing doesn't exist, then it's gonna fail!
//func TestAllCommandsHaveDocFile(t *testing.T) {}

// TODO: not handled: aliases
func collectAllCommandNames(app *cli.App) []string {
	commandNames := []string{app.Name}
	for _, subcmd := range wfapp.App.Commands { // First round is weird because `*cli.App` isn't itself a `*cli.Command`.
		collectSubcommandNames(app.Name, subcmd, &commandNames)
	}
	return commandNames
}

func collectSubcommandNames(pth string, cmd *cli.Command, appendme *[]string) {
	// `cmd.FullName()`, though it says it should do what we want, appears to be unimplemented.  So we carry down `pth` ourselves.
	pth += " " + cmd.Name
	*appendme = append(*appendme, pth)
	for _, subcmd := range cmd.Subcommands {
		collectSubcommandNames(pth, subcmd, appendme)
	}
}
