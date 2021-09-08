package wfapi

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
)

func TestParseCatalog(t *testing.T) {
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
}
`

	catLin := CatalogLineageEnvelope{}
	_, err := ipld.Unmarshal([]byte(serial), json.Decode, &catLin, TypeSystem.TypeByName("CatalogLineageEnvelope"))
	qt.Assert(t, err, qt.IsNil)

	reserial, err := ipld.Marshal(json.Encode, &catLin, TypeSystem.TypeByName("CatalogLineageEnvelope"))
	qt.Assert(t, err, qt.IsNil)

	qt.Assert(t, string(reserial), qt.CmpEquals(), serial)
}
