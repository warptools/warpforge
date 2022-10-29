package wfapi

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/ipld/go-ipld-prime"
	_ "github.com/ipld/go-ipld-prime/codec/dagcbor"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/ipld/go-ipld-prime/printer"
	"github.com/warpfork/go-testmark"

	_ "github.com/warpfork/warpforge/pkg/testutil"
)

func TestPlotParseFixtures(t *testing.T) {
	doc, err := testmark.ReadFile("../examples/210-plot-parse/example-plots.md")
	if err != nil {
		t.Fatalf("spec file parse failed?!: %s", err)
	}

	// Data hunk in this spec file are in "directories" of a test scenario each.
	doc.BuildDirIndex()
	for _, dir := range doc.DirEnt.ChildrenList {
		t.Run(dir.Name, func(t *testing.T) {
			// Each "directory" should contain at least "plot".
			switch {
			case dir.Children["plot"] != nil:
				// Nab the bytes.
				serial := dir.Children["plot"].Hunk.Body

				// Unmarshal.  Assert it works.
				t.Run("unmarshal", func(t *testing.T) {
					plotCapsule := PlotCapsule{}
					n, err := ipld.Unmarshal(serial, json.Decode, &plotCapsule, TypeSystem.TypeByName("PlotCapsule"))
					qt.Assert(t, err, qt.IsNil)

					// If there was data about debug forms, check that matches.
					if dir.Children["plot.debug"] != nil {
						printed := printer.Sprint(n)
						qt.Assert(t, printed+"\n", qt.CmpEquals(), string(dir.Children["plot.debug"].Hunk.Body))
					}

					// Remarshal.  Assert it works.
					t.Run("remarshal", func(t *testing.T) {
						reserial, err := ipld.Marshal(json.Encode, &plotCapsule, TypeSystem.TypeByName("PlotCapsule"))
						qt.Assert(t, err, qt.IsNil)

						// And assert it's string equal.
						t.Run("exact-match", func(t *testing.T) {
							t.Skip("needs https://github.com/ipld/go-ipld-prime/pull/239")
							qt.Assert(t, string(reserial), qt.CmpEquals(), string(serial))
						})
					})
				})
			}
		})
	}
}
