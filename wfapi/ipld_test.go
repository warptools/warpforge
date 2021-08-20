package wfapi

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
)

// Critical lament with this testing style: this validation doesn't happen before other tests.
// We also couldn't do it during the package init, because of lack of ordering there.
// Uff.  lol.
// The consequence is that if you have an invalid schema, you might hear about it from obscure bindnode errors that should be unreachable for a valid schema.

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
					"linux-amd64": "tar:asdf",
					"darwin-amd64": "tar:qwer"
				},
				"metadata": {
					"whee": "yay"
				}
			},
			{
				"name": "v2.0",
				"items": {
					"linux-amd64": "tar:zonk",
					"darwin-amd64": "tar:bonk"
				},
				"metadata": {
					"whee": "yahoo"
				}
			}
		]
	}
}`

	catLin := CatalogLineageEnvelope{}
	_, err := ipld.Unmarshal([]byte(serial), json.Decode, &catLin, TypeSystem.TypeByName("CatalogLineageEnvelope"))
	qt.Assert(t, err, qt.IsNil)
}