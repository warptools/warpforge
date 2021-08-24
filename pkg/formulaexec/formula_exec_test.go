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

func TestPack(t *testing.T) {
	serial := `{
	"formula": {
		"inputs": {
			"/": "ware:tar:47Yg1Sdq21rPyDw9X9sCmRubQUADhFKe9G7qZCJRe61RhWPCxcQysCFzyCHffBKRjB"
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

	err = Exec(frmAndCtx)
	qt.Assert(t, err, qt.IsNil)
}

func TestDirMount(t *testing.T) {
	serial := `{
	"formula": {
		"inputs": {
			"/": "ware:tar:47Yg1Sdq21rPyDw9X9sCmRubQUADhFKe9G7qZCJRe61RhWPCxcQysCFzyCHffBKRjB",
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

	err = Exec(frmAndCtx)
	qt.Assert(t, err, qt.IsNil)
}


func TestContextWarehouse(t *testing.T) {
	serial := `{
	"formula": {
		"inputs": {
			"/": "ware:tar:47Yg1Sdq21rPyDw9X9sCmRubQUADhFKe9G7qZCJRe61RhWPCxcQysCFzyCHffBKRjB",
			"/empty": "ware:tar:fake"
		},
		"action": {
			"exec": {
				"command": ["/bin/sh", "-c", "ls -al /test"]
			}
		},
		"outputs": {
		}
	},
	"context": {
		"warehouses": {
			"tar:fake": "file:///fake/file.tar.gz"
		}
	}
}`
	frmAndCtx := wfapi.FormulaAndContext{}
	_, err := ipld.Unmarshal([]byte(serial), json.Decode, &frmAndCtx, wfapi.TypeSystem.TypeByName("FormulaAndContext"))
	qt.Assert(t, err, qt.IsNil)

	err = Exec(frmAndCtx)

	// this should error on the rio unpack since the .tar.gz file does not exist
	qt.Assert(t, err, qt.IsNotNil)
}
