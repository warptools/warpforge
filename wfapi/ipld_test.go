package wfapi

import (
	"bytes"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/dagjson"
	"github.com/ipld/go-ipld-prime/node/bindnode"

	_ "github.com/warptools/warpforge/pkg/testutil"
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

// Example of IPLD WareID repr->struct
func TestWareID_ReprToStruct_Unmarshal(t *testing.T) {
	wareRepr := "tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9"
	wid := WareID{
		Hash:     "4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9",
		Packtype: "tar",
	}
	result := WareID{}
	// janky need to quote string before feeding it to json interpreter
	_, err := ipld.Unmarshal([]byte(`"`+wareRepr+`"`), dagjson.Decode, &result, TypeSystem.TypeByName("WareID"))
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, result, qt.DeepEquals, wid)
}

// Example of IPLD WareID repr->struct
func TestWareID_ReprToStruct_Build(t *testing.T) {
	wareRepr := "tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9"
	wid := WareID{
		Hash:     "4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9",
		Packtype: "tar",
	}

	np := bindnode.Prototype(&WareID{}, TypeSystem.TypeByName("WareID"))
	npr := np.Representation()
	nb := npr.NewBuilder()
	err := nb.AssignString(wareRepr)
	qt.Assert(t, err, qt.IsNil)
	node := nb.Build()
	result := bindnode.Unwrap(node).(*WareID)
	qt.Assert(t, result, qt.DeepEquals, &wid)
}

// Example of IPLD WareID struct -> repr
func TestWareID_StructToRepr_Encode(t *testing.T) {
	wareRepr := "tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9"
	wid := WareID{
		Hash:     "4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9",
		Packtype: "tar",
	}
	node := bindnode.Wrap(&wid, TypeSystem.TypeByName("WareID"))
	buf := bytes.NewBuffer([]byte{})
	err := dagjson.Encode(node.Representation(), buf)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, buf.String(), qt.Equals, `"`+wareRepr+`"`)
}

// Example of IPLD WareID struct -> repr
func TestWareID_StructToRepr_AsString(t *testing.T) {
	wareRepr := "tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9"
	wid := WareID{
		Hash:     "4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9",
		Packtype: "tar",
	}
	node := bindnode.Wrap(&wid, TypeSystem.TypeByName("WareID"))
	result, err := node.Representation().AsString()
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, result, qt.Equals, wareRepr)
}
