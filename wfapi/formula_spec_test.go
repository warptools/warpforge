package wfapi

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	_ "github.com/ipld/go-ipld-prime/codec/dagcbor"
	"github.com/ipld/go-ipld-prime/codec/json"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/printer"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/warpfork/go-testmark"

	_ "github.com/warptools/warpforge/pkg/testutil"
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
					n, err := ipld.Unmarshal(serial, json.Decode, &frmAndCtx, TypeSystem.TypeByName("FormulaAndContext"))
					qt.Assert(t, err, qt.IsNil)

					// If there was data about debug forms, check that matches.
					if dir.Children["formula.debug"] != nil {
						printed := printer.Sprint(n)
						qt.Assert(t, printed+"\n", qt.CmpEquals(), string(dir.Children["formula.debug"].Hunk.Body))
					}

					// Remarshal.  Assert it works.
					t.Run("remarshal", func(t *testing.T) {
						reserial, err := ipld.Marshal(json.Encode, &frmAndCtx, TypeSystem.TypeByName("FormulaAndContext"))
						qt.Assert(t, err, qt.IsNil)

						// And assert it's string equal.
						t.Run("exact-match", func(t *testing.T) {
							qt.Assert(t, string(reserial), qt.CmpEquals(), string(serial))
						})
					})

					// If there's a link datum: Create a CID of the formula and see what's up.
					// (Also encodes, implicitly.  So will probably fail if the above was broken.)
					// Path into the Formula before doing this -- we don't want to hash the context or the envelope type.
					if dir.Children["cid"] != nil {
						nCapsule, _ := n.LookupByString("formula")
						nFormula, _ := nCapsule.LookupByString("Formula")
						lsys := cidlink.DefaultLinkSystem()
						lnk, err := lsys.ComputeLink(cidlink.LinkPrototype{cid.Prefix{
							Version:  1,    // Usually '1'.
							Codec:    0x71, // 0x71 means "dag-cbor" -- See the multicodecs table: https://github.com/multiformats/multicodec/
							MhType:   0x20, // 0x20 means "sha2-384" -- See the multicodecs table: https://github.com/multiformats/multicodec/
							MhLength: 48,   // sha2-384 hash has a 48-byte sum.
						}}, nFormula.(schema.TypedNode).Representation())
						qt.Assert(t, err, qt.IsNil)
						expect := strings.TrimSpace(string(dir.Children["cid"].Hunk.Body))
						cid, err := lnk.(cidlink.Link).StringOfBase('z')
						qt.Assert(t, err, qt.IsNil)
						qt.Assert(t, expect, qt.Equals, cid)
					}
				})
			case dir.Children["runrecord"] != nil:
				// TODO
			}
		})
	}
}
