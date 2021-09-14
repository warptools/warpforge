package formulaexec

import (
	"fmt"
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
			"/": "ware:tar:7P8nq1YY361BSEvgsSU3gu4ot1U5ieiFey2XyvMoTM7Mhwg3mo8aV2KyGwwrKRLtxS"
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

	rr, err := Exec(frmAndCtx)
	qt.Assert(t, err, qt.IsNil)

	rr_serial, err := ipld.Marshal(json.Encode, &rr, wfapi.TypeSystem.TypeByName("RunRecord"))
	qt.Assert(t, err, qt.IsNil)
	fmt.Println(string(rr_serial))

}

func TestPack(t *testing.T) {
	serial := `{
	"formula": {
		"inputs": {
			"/": "ware:tar:7P8nq1YY361BSEvgsSU3gu4ot1U5ieiFey2XyvMoTM7Mhwg3mo8aV2KyGwwrKRLtxS"
		},
		"action": {
			"exec": {
				"command": ["/bin/sh", "-c", "mkdir /out; echo hello from warpforge! > /out/test"]
			}
		},
		"outputs": {
			"test": {
				"from": "/out",
				"packtype": "tar"
			},

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

	rr, err := Exec(frmAndCtx)
	qt.Assert(t, err, qt.IsNil)

	rr_serial, err := ipld.Marshal(json.Encode, &rr, wfapi.TypeSystem.TypeByName("RunRecord"))
	qt.Assert(t, err, qt.IsNil)
	fmt.Println(string(rr_serial))
}

func TestDirMount(t *testing.T) {
	serial := `{
	"formula": {
		"inputs": {
			"/": "ware:tar:7P8nq1YY361BSEvgsSU3gu4ot1U5ieiFey2XyvMoTM7Mhwg3mo8aV2KyGwwrKRLtxS",
			"/work": "mount:type:."
		},
		"action": {
			"exec": {
				"command": ["/bin/sh", "-c", "ls -al /work"]
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

	_, err = Exec(frmAndCtx)
	qt.Assert(t, err, qt.IsNil)
}
