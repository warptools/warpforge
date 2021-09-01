package wfapi

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
)

func TestParsePlot(t *testing.T) {
	serial := `{
	"inputs": {
		"test": "ware:tar:foo"
	},
	"steps": {
		"one": {
			"protoformula": {
				"inputs": {
					"/": "ware:tar:hash"
				},
				"action": {
					"exec": {
						"command": [
							"/bin/echo",
							"hi"
						]
					}
				},
				"outputs": {
					"test": {
						"from": "/",
						"packtype": "tar"
					}
				}
			}
		}
	},
	"outputs": {
		"test": "one:test"
	}
}
`

	p := Plot{}
	_, err := ipld.Unmarshal([]byte(serial), json.Decode, &p, TypeSystem.TypeByName("Plot"))
	qt.Assert(t, err, qt.IsNil)

	reserial, err := ipld.Marshal(json.Encode, &p, TypeSystem.TypeByName("Plot"))
	qt.Assert(t, err, qt.IsNil)

	qt.Assert(t, string(reserial), qt.CmpEquals(), serial)
}
