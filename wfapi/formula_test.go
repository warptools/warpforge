package wfapi

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"

	_ "github.com/warpfork/warpforge/pkg/testutil"
)

func TestParseFormulaAndContext(t *testing.T) {
	serial := `{
	"formula": {
		"formula.v1": {
			"inputs": {
				"/mount/path": "ware:tar:qwerasdf",
				"$ENV_VAR": "literal:hello",
				"/more/mounts": {
					"basis": "ware:tar:fghjkl",
					"filters": {
						"uid": "10"
					}
				}
			},
			"action": {
				"exec": {
					"command": [
						"/bin/bash",
						"-c",
						"echo hey there"
					]
				}
			},
			"outputs": {
				"theoutputlabel": {
					"from": "/collect/here",
					"packtype": "tar"
				},
				"another": {
					"from": "$VAR"
				}
			}
		}
	},
	"context": {
		"context.v1": {
			"warehouses": {
				"tar:qwerasdf": "ca+file:///somewhere/",
				"tar:fghjkl": "ca+file:///elsewhere/"
			}
		}
	}
}
`

	frmAndCtx := FormulaAndContext{}
	_, err := ipld.Unmarshal([]byte(serial), json.Decode, &frmAndCtx, TypeSystem.TypeByName("FormulaAndContext"))
	qt.Assert(t, err, qt.IsNil)

	ports := []SandboxPort{
		{SandboxPath: func() *SandboxPath { v := SandboxPath("mount/path"); return &v }()},
		{SandboxVar: func() *SandboxVar { v := SandboxVar("ENV_VAR"); return &v }()},
		{SandboxPath: func() *SandboxPath { v := SandboxPath("more/mounts"); return &v }()},
	}
	inputs := frmAndCtx.Formula.Formula.Inputs
	qt.Assert(t, inputs.Keys, qt.DeepEquals, ports)
	// Okay, this is going to look a little funny, and it's because we've got another upstream bug to fix.
	// `inputs.Values[ports[0]]` isn't enough ... because SandboxPort has pointers in it... so, key equality fail.
	// But if we use the values from the keys slice, those are pointer equality all the way down, so it works.
	qt.Check(t, inputs.Values[inputs.Keys[0]], qt.DeepEquals,
		FormulaInput{FormulaInputSimple: &FormulaInputSimple{WareID: &WareID{"tar", "qwerasdf"}}},
	)
	qt.Check(t, inputs.Values[inputs.Keys[1]], qt.DeepEquals,
		FormulaInput{FormulaInputSimple: &FormulaInputSimple{Literal: func() *Literal { v := Literal("hello"); return &v }()}},
	)
	qt.Check(t, inputs.Values[inputs.Keys[2]], qt.DeepEquals,
		FormulaInput{FormulaInputComplex: &FormulaInputComplex{
			Basis: FormulaInputSimple{WareID: &WareID{"tar", "fghjkl"}},
			Filters: FilterMap{
				Keys:   []string{"uid"},
				Values: map[string]string{"uid": "10"},
			},
		}},
	)

	reserial, err := ipld.Marshal(json.Encode, &frmAndCtx, TypeSystem.TypeByName("FormulaAndContext"))
	qt.Assert(t, err, qt.IsNil)

	qt.Assert(t, string(reserial), qt.CmpEquals(), serial)
}

func TestParseRunRecord(t *testing.T) {
	serial := `{
	"guid": "asefjghr-34jg5nhj-12jfb5jk",
	"time": 1245869935,
	"formulaID": "Qfm2kJElwkJElfkej5gH",
	"exitcode": 0,
	"results": {
		"theoutputlabel": "ware:tar:qwerasdferguih",
		"another": "literal:some content hello yes"
	}
}
`

	rr := RunRecord{}
	_, err := ipld.Unmarshal([]byte(serial), json.Decode, &rr, TypeSystem.TypeByName("RunRecord"))
	qt.Assert(t, err, qt.IsNil)

	reserial, err := ipld.Marshal(json.Encode, &rr, TypeSystem.TypeByName("RunRecord"))
	qt.Assert(t, err, qt.IsNil)

	qt.Assert(t, string(reserial), qt.CmpEquals(), serial)
}
