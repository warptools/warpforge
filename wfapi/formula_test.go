package wfapi

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/ipld/go-ipld-prime/node/bindnode"
)

func TestParseFormulaAndContext(t *testing.T) {
	serial := `{
	"formula": {
		"inputs": {
			"/mount/path": "tar:qwerasdf",
			"$ENV_VAR": "literal:hello"
		},
		"action": {
			"exec": {
				"command": ["/bin/bash", "-c", "echo hey there"]
			}
		},
		"outputs": {}
	},
	"context": {
		"warehouses": {
			"tar:qwerasdf": "ca+file:///somewhere/"
		}
	}
}`

	np := bindnode.Prototype((*FormulaAndContext)(nil), TypeSystem.TypeByName("FormulaAndContext"))
	nb := np.Representation().NewBuilder()
	err := json.Decode(nb, strings.NewReader(serial))
	qt.Assert(t, err, qt.IsNil)
	n := bindnode.Unwrap(nb.Build()).(*FormulaAndContext)
	_ = n
}
