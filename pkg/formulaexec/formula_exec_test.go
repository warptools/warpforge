package formulaexec

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"

	"github.com/warpfork/warpforge/wfapi"
)

func TestEcho(t *testing.T) {
	serial := `{
	"formula": {
		"inputs": {
			"/": "ware:tar:47Yg1Sdq21rPyDw9X9sCmRubQUADhFKe9G7qZCJRe61RhWPCxcQysCFzyCHffBKRjB"
		},
		"action": {
			"exec": {
				"command": ["/bin/sh", "-c", "echo hello from warpforge!"]
			}
		},
		"outputs": {
		}
	},
	"context": {
		"warehouses": {
		}
	}
}`
	frmAndCtx := wfapi.FormulaAndContext{}
	_, err := ipld.Unmarshal([]byte(serial), json.Decode, &frmAndCtx, wfapi.TypeSystem.TypeByName("FormulaAndContext"))
	qt.Assert(t, err, qt.IsNil)

	err = Exec(frmAndCtx)
	qt.Assert(t, err, qt.IsNil)
}
