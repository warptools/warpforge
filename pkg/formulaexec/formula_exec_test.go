package formulaexec

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"

	"github.com/warpfork/go-testmark"
	_ "github.com/warpfork/warpforge/pkg/testutil"
	"github.com/warpfork/warpforge/pkg/workspace"
	"github.com/warpfork/warpforge/wfapi"
)

// constructs a custom workspace set containing only this project's .warpforge dir (contains catalog)
func getTestWorkspaceStack(t *testing.T) workspace.WorkspaceSet {
	pwd, err := os.Getwd()
	qt.Assert(t, err, qt.IsNil)
	projWs, err := workspace.OpenWorkspace(os.DirFS("/"), filepath.Join(pwd[1:], "../../"))
	qt.Assert(t, err, qt.IsNil)

	wss := workspace.WorkspaceSet{
		Home: projWs,
		Root: projWs,
		Stack: []*workspace.Workspace{
			projWs,
		},
	}
	return wss
}

func configureEnvironment(t *testing.T) {
	pwd, err := os.Getwd()
	qt.Assert(t, err, qt.IsNil)
	err = os.Setenv("WARPFORGE_PATH", filepath.Join(pwd, "../../plugins"))
	qt.Assert(t, err, qt.IsNil)
	err = os.Setenv("HOME", filepath.Join(pwd, "../../"))
	qt.Assert(t, err, qt.IsNil)
}

func evaluateDoc(t *testing.T, doc *testmark.Document) {
	// Data hunk in this spec file are in "directories" of a test scenario each.
	doc.BuildDirIndex()
	for _, dir := range doc.DirEnt.ChildrenList {
		t.Run(dir.Name, func(t *testing.T) {

			// Each "directory" should contain at least either "formula" or "runrecord".
			switch {
			case dir.Children["formula"] != nil:
				// Nab the bytes.
				serial := dir.Children["formula"].Hunk.Body

				t.Run("exec-formula", func(t *testing.T) {
					ctx := context.Background()
					frmAndCtx := wfapi.FormulaAndContext{}
					_, err := ipld.Unmarshal(serial, json.Decode, &frmAndCtx, wfapi.TypeSystem.TypeByName("FormulaAndContext"))
					qt.Assert(t, err, qt.IsNil)

					config := wfapi.FormulaExecConfig{}

					pwd, err := os.Getwd()
					qt.Assert(t, err, qt.IsNil)
					projWs, err := workspace.OpenWorkspace(os.DirFS("/"), filepath.Join(pwd[1:], "../../"))
					qt.Assert(t, err, qt.IsNil)

					rr, err := Exec(ctx, projWs, frmAndCtx, config)
					qt.Assert(t, err, qt.IsNil)

					rrSerial, err := ipld.Marshal(json.Encode, &rr, wfapi.TypeSystem.TypeByName("RunRecord"))
					qt.Assert(t, err, qt.IsNil)

					fmt.Println(string(rrSerial))

					// if an example RunRecord is present, compare it
					if dir.Children["runrecord"] != nil {
						rrExample := wfapi.RunRecord{}
						_, err := ipld.Unmarshal(dir.Children["runrecord"].Hunk.Body, json.Decode, &rrExample, wfapi.TypeSystem.TypeByName("RunRecord"))
						qt.Assert(t, err, qt.IsNil)

						// ensure the non-deterministic parts of the runrecord are set to known values
						rr.Guid = "abcd"
						rrExample.Guid = "abcd"
						rr.Time = 1234
						rrExample.Time = 1234
						// assert the example is correct
						qt.Assert(t, rr, qt.CmpEquals(), rrExample)
					}

				})
			}
		})
	}
}

// Test example formulas.
func TestFormulaExecFixtures(t *testing.T) {
	doc, err := testmark.ReadFile("../../examples/110-formula-usage/example-formula-exec.md")
	if err != nil {
		t.Fatalf("spec file parse failed?!: %s", err)
	}

	configureEnvironment(t)
	evaluateDoc(t, doc)
}

func TestFormulaScriptFixtures(t *testing.T) {
	doc, err := testmark.ReadFile("../../examples/110-formula-usage/example-formula-script.md")
	if err != nil {
		t.Fatalf("spec file parse failed?!: %s", err)
	}

	configureEnvironment(t)
	evaluateDoc(t, doc)
}
