package plotexec

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

	var wss workspace.WorkspaceSet = []*workspace.Workspace{
		projWs,
	}
	return wss
}

// Test example plots.
func TestFormulaExecFixtures(t *testing.T) {
	doc, err := testmark.ReadFile("../../examples/220-plot-usage/example-plot-exec.md")
	if err != nil {
		t.Fatalf("spec file parse failed?!: %s", err)
	}

	// override the path to required binaries
	pwd, err := os.Getwd()
	qt.Assert(t, err, qt.IsNil)
	err = os.Setenv("WARPFORGE_PATH", filepath.Join(pwd, "../../plugins"))
	qt.Assert(t, err, qt.IsNil)

	// Data hunk in this spec file are in "directories" of a test scenario each.
	doc.BuildDirIndex()
	for _, dir := range doc.DirEnt.ChildrenList {
		t.Run(dir.Name, func(t *testing.T) {
			switch {
			case dir.Children["plot"] != nil:
				// Nab the bytes.
				serial := dir.Children["plot"].Hunk.Body

				t.Run("exec-plot", func(t *testing.T) {
					ctx := context.Background()
					plotCapsule := wfapi.PlotCapsule{}
					_, err := ipld.Unmarshal(serial, json.Decode, &plotCapsule, wfapi.TypeSystem.TypeByName("PlotCapsule"))
					qt.Assert(t, err, qt.IsNil)
					qt.Assert(t, plotCapsule.Plot, qt.IsNotNil)

					// determine step ordering and compare to example
					steps, err := OrderStepsAll(ctx, *plotCapsule.Plot)
					qt.Assert(t, err, qt.IsNil)
					if dir.Children["order"] != nil {
						qt.Assert(t, string(dir.Children["order"].Hunk.Body), qt.CmpEquals(), fmt.Sprintf("%s\n", steps))
					}

					wss := getTestWorkspaceStack(t)
					config := wfapi.PlotExecConfig{
						Recursive: true,
					}
					results, err := Exec(ctx, wss, plotCapsule, config)
					qt.Assert(t, err, qt.IsNil)

					// print the serialized results, this can be copied into the testmark file
					resultsSerial, err := ipld.Marshal(json.Encode, &results, wfapi.TypeSystem.TypeByName("PlotResults"))
					qt.Assert(t, err, qt.IsNil)
					fmt.Println(string(resultsSerial))

					// if an example PlotResults is present, compare it
					if dir.Children["plotresults"] != nil {
						resultsExample := wfapi.PlotResults{}
						_, err := ipld.Unmarshal(dir.Children["plotresults"].Hunk.Body, json.Decode, &resultsExample, wfapi.TypeSystem.TypeByName("PlotResults"))
						qt.Assert(t, err, qt.IsNil)

						qt.Assert(t, resultsExample, qt.CmpEquals(), results)
					}

				})
			}
		})
	}
}

// Test that a plot with cyclic inputs fails. This should be detected as
// a cylic graph and throw an error.
func TestCycleFails(t *testing.T) {
	serial := `{
	"inputs": {},
	"steps": {
		"zero": {
			"protoformula": {
				"inputs": {
					"/": "pipe:one:out"
				},
				"action": {
					"exec": {
						"command": [
							"/bin/echo"
						]
					}
				},
				"outputs": {
					"out": {
						"from": "/",
						"packtype": "tar"
					}
				}
			}
		},
		"one": {
			"protoformula": {
				"inputs": {
					"/": "pipe:zero:out",
				},
				"action": {
					"exec": {
						"command": [
							"/bin/echo"
						]
					}
				},
				"outputs": {
					"out": {
						"from": "/",
						"packtype": "tar"
					}
				}
			}
		}

	},
	"outputs": {}
}
`
	ctx := context.Background()
	p := wfapi.Plot{}
	_, err := ipld.Unmarshal([]byte(serial), json.Decode, &p, wfapi.TypeSystem.TypeByName("Plot"))
	qt.Assert(t, err, qt.IsNil)

	// this will fail due to a dependency cycle between steps zero and one
	_, err = OrderSteps(ctx, p)
	qt.Assert(t, err, qt.IsNotNil)
}
