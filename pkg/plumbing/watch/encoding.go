package watch

import (
	"io"

	"github.com/ipld/go-ipld-prime/codec"
	"github.com/ipld/go-ipld-prime/codec/dagjson"
	"github.com/ipld/go-ipld-prime/datamodel"
	rfmtjson "github.com/polydawn/refmt/json"
	"github.com/serum-errors/go-serum"

	"github.com/warptools/warpforge/wfapi"
)

var encodeOptions dagjson.EncodeOptions = dagjson.EncodeOptions{
	EncodeLinks: false,
	EncodeBytes: false,
	MapSortMode: codec.MapSortMode_None,
}

var decodeOptions dagjson.DecodeOptions = dagjson.DecodeOptions{
	ParseLinks:         false,
	ParseBytes:         false,
	DontParseBeyondEnd: true, // This is critical for streaming over a socket
}

func encode(n datamodel.Node, w io.Writer, opt rfmtjson.EncodeOptions) error {
	err := dagjson.Marshal(n, rfmtjson.NewEncoder(w, opt), encodeOptions)
	if err != nil {
		return serum.Error(wfapi.ECodeSerialization, serum.WithCause(err),
			serum.WithMessageLiteral("watch encoder failed"),
		)
	}
	return nil
}

// Errors:
//
//   - warpforge-error-serialization --
func Encoder(n datamodel.Node, w io.Writer) error {
	return encode(n, w, rfmtjson.EncodeOptions{
		Line:   []byte{},
		Indent: []byte{},
	})
}

// Errors:
//
//   - warpforge-error-serialization --
func PrettyEncoder(n datamodel.Node, w io.Writer) error {
	return encode(n, w, rfmtjson.EncodeOptions{
		Line:   []byte{'\n'},
		Indent: []byte{'\t'},
	})
}

// Errors:
//
//   - warpforge-error-serialization --
func Decoder(na datamodel.NodeAssembler, r io.Reader) error {
	err := decodeOptions.Decode(na, r)
	if err != nil {
		return serum.Error(wfapi.ECodeSerialization, serum.WithCause(err),
			serum.WithMessageLiteral("watch decoder failed"),
		)
	}
	return nil
}
