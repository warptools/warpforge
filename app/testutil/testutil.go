package testutil

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/serum-errors/go-serum"
	"github.com/warpfork/go-testmark"
	"github.com/warpfork/go-testmark/testexec"

	wfapp "github.com/warptools/warpforge/app"
	"github.com/warptools/warpforge/pkg/testutil"
)

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

func TestFileContainingTestmarkexec(t *testing.T, fileName string, workDir *string) {
	t.Logf("loading test file: %q", fileName)
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
			execFn := buildExecFn(t, filepath.Join(pwd, "../")) // FIXME this depends on which package is being tested
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

// Replace non-deterministic values of JSON runrecord to allow for deterministic comparison
func cleanOutput(str string) string {
	// replace guid
	matcher := regexp.MustCompile(`"guid": "[a-zA-Z0-9]{8}-[a-zA-Z0-9]{4}-[a-zA-Z0-9]{4}-[a-zA-Z0-9]{4}-[a-zA-Z0-9]{12}"`)
	str = matcher.ReplaceAllString(str, `"guid": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"`)

	// replace time
	matcher = regexp.MustCompile(`"time": [0-9]+`)
	str = matcher.ReplaceAllString(str, `"time": "22222222222"`)

	// replace tmp path
	matcher = regexp.MustCompile(`/tmp/go-build.*/warpforge.test`)
	str = matcher.ReplaceAllString(str, `warpforge`)

	// return value with whitespace trimmed
	return strings.TrimSpace(str)
}

// Warning!  Impure function!  Cannot safely be used in parallel!
// This mutates the CLI app object to wires the IO streams.
// Also, it uses `os.Chdir` on this process (because we're "emulating a shell" rather than making subprocesses, whee).
func buildExecFn(t *testing.T, projPath string) func([]string, io.Reader, io.Writer, io.Writer) (int, error) {
	return func(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		bufout, buferr := &bytes.Buffer{}, &bytes.Buffer{}
		var testout io.Writer = bufout
		if stdout != nil {
			testout = io.MultiWriter(stdout, bufout)
		}
		var testerr io.Writer = bufout
		if stderr != nil {
			testerr = io.MultiWriter(stderr, buferr)
		}

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

		wd, err := os.Getwd()
		if err != nil {
			panic("failed to find working directory")
		}
		t.Log("╲╱╲╱╲╱╲╱╲╱╲╱╲╱╲╱╲╱╲╱╲╱╲╱╲╱╲╱╲╱╲╱╲╱╲╱╲╱╲╱")
		t.Logf("Working Directory: %q", wd)
		// filepath.Walk(wd, func(path string, d os.FileInfo, err error) error {
		// 	t.Logf("path: %s", path)
		// 	return nil
		// })

		wfapp.App.Reader = stdin
		wfapp.App.Writer = testout
		wfapp.App.ErrWriter = testerr
		err = wfapp.App.Run(args)

		exitCode := 0
		if err != nil {
			exitCode = 1 // TODO more rich exit code selection -- this should happen in app package or somewhere more shared
		}

		t.Logf("Args: %v", args)
		for err != nil {
			t.Logf("Code: %s", serum.Code(err))
			t.Logf("Message: %s", serum.Message(err))
			t.Logf("Details: %v", serum.Details(err))
			err = errors.Unwrap(err)
			if err != nil {
				t.Logf("caused by:")
			}
		}
		t.Logf("==============")
		t.Logf("⌄⌄⌄ stdout ⌄⌄⌄\n%s", string(bufout.Bytes()))
		t.Logf("⌃⌃⌃ stdout ⌃⌃⌃")
		t.Logf("==============")
		t.Logf("⌄⌄⌄ stderr ⌄⌄⌄\n%s", string(buferr.Bytes()))
		t.Logf("⌃⌃⌃ stderr ⌃⌃⌃")
		t.Logf("==============")
		return exitCode, nil
	}
}

func assertFn(t *testing.T, actual, expect string) {
	actual = cleanOutput(actual)
	expect = cleanOutput(expect)
	qt.Assert(t, actual, qt.Equals, expect)
}
