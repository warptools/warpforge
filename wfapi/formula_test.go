package wfapi

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
)

func TestParseFormulaAndContext(t *testing.T) {
	serial := `{
	"formula": {
		"inputs": {
			"/mount/path": "ware:tar:qwerasdf",
			"$ENV_VAR": "literal:hello",
			"/more/mounts": {"basis": "ware:tar:fghjkl", "filters":{"uid":"10"}}
		},
		"action": {
			"exec": {
				"command": ["/bin/bash", "-c", "echo hey there"]
			}
		},
		"outputs": {
			"theoutputlabel": {
				"from": "/collect/here",
				"packtype": "tar"
			},
			"another": {
				"from": "$VAR",
			}
		}
	},
	"context": {
		"warehouses": {
			"tar:qwerasdf": "ca+file:///somewhere/",
			"tar:fghjkl": "ca+file:///elsewhere/"
		}
	}
}`

	frmAndCtx := FormulaAndContext{}
	_, err := ipld.Unmarshal([]byte(serial), json.Decode, &frmAndCtx, TypeSystem.TypeByName("FormulaAndContext"))
	qt.Assert(t, err, qt.IsNil)

	ports := []SandboxPort{
		SandboxPort{SandboxPath: func() *SandboxPath { v := SandboxPath("mount/path"); return &v }()},
		SandboxPort{VariableName: func() *VariableName { v := VariableName("ENV_VAR"); return &v }()},
		SandboxPort{SandboxPath: func() *SandboxPath { v := SandboxPath("more/mounts"); return &v }()},
	}
	inputs := frmAndCtx.Formula.Inputs
	qt.Assert(t, inputs.Keys, qt.DeepEquals, ports)
	// Okay, this is going to look a little funny, and it's because we've got another upstream bug to fix.
	// `inputs.Values[ports[0]]` isn't enough ... because SandboxPort has pointers in it... so, key equality fail.
	// But if we use the values from the keys slice, those are pointer equality all the way down, so it works.
	qt.Check(t, inputs.Values[inputs.Keys[0]], qt.DeepEquals,
		FormulaInput{FormulaInputSimple: &FormulaInputSimple{WareID: &WareID{"tar", "qwerasdf"}}},
	)
	qt.Check(t, inputs.Values[inputs.Keys[1]], qt.DeepEquals,
		FormulaInput{FormulaInputSimple: &FormulaInputSimple{Literal: func() *string { v := "hello"; return &v }()}},
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
}
