package wfapi

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/warpfork/go-testmark"
)

func TestFormulaParseFixtures(t *testing.T) {
	doc, err := testmark.ReadFile("../examples/100-formula-parse/example-formulas.md")
	if err != nil {
		t.Fatalf("spec file parse failed?!: %s", err)
	}

	// Data hunk in this spec file are in "directories" of a test scenario each.
	doc.BuildDirIndex()
	for _, dir := range doc.DirEnt.ChildrenList {
		t.Run(dir.Name, func(t *testing.T) {
			// Each "directory" should contain at least either "formula" or "runrecord".
			switch {
			case dir.Children["formula"] != nil:
				// Nab the bytes.
				serial := dir.Children["formula"].Hunk.Body

				// Unmarshal.  Assert it works.
				t.Run("unmarshal", func(t *testing.T) {
					frmAndCtx := FormulaAndContext{}
					_, err := ipld.Unmarshal(serial, json.Decode, &frmAndCtx, TypeSystem.TypeByName("FormulaAndContext"))
					qt.Assert(t, err, qt.IsNil)

					// Remarshal.  Assert it works.
					t.Run("remarshal", func(t *testing.T) {
						reserial, err := ipld.Marshal(json.Encode, &frmAndCtx, TypeSystem.TypeByName("FormulaAndContext"))
						qt.Assert(t, err, qt.IsNil)

						// And assert it's string equal.
						t.Run("exact-match", func(t *testing.T) {
							qt.Assert(t, string(reserial), qt.CmpEquals(), string(serial))
						})
					})
				})
			case dir.Children["runrecord"] != nil:
				// TODO
			}
		})
	}
}
