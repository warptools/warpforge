package wfapi

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/ipld/go-ipld-prime/node/bindnode"
)

func TestTypeSystemCompiles(t *testing.T) {
	if errs := TypeSystem.ValidateGraph(); errs != nil {
		qt.Assert(t, errs, qt.IsNil)
	}
}

func TestCatalogSerialForm(t *testing.T) {
	serial := `{
	"catalogLineage": {
		"name": "foo.org/bar",
		"metadata": {
			"whatever": "yo"
		},
		"releases": [
			{
				"name": "v1.0",
				"items": {
					"linux-amd64": "tar:asdf"
					"darwin-amd64": "tar:qwer",
				},
				"metadata": {
					"whee": "yay"
				}
			},
			{
				"name": "v2.0",
				"items": {
					"linux-amd64": "tar:zonk"
					"darwin-amd64": "tar:bonk",
				},
				"metadata": {
					"whee": "yahoo"
				}
			}
		]
	}
}`

	np := bindnode.Prototype((*CatalogLineageEnvelope)(nil), TypeSystem.TypeByName("CatalogLineageEnvelope"))
	nb := np.Representation().NewBuilder()
	err := json.Decode(nb, strings.NewReader(serial))
	qt.Assert(t, err, qt.IsNil)
	// n := bindnode.Unwrap(nb.Build()).(*CatalogLineageEnvelope)
	// _ = n.Value.(CatalogLineage) // doesn't work yet -- more bindnode features needed
}
