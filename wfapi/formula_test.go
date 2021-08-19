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
}
