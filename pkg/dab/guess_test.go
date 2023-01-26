package dab

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/serum-errors/go-serum"

	"github.com/warptools/warpforge/wfapi"
)

func TestGuessDocumentType(t *testing.T) {
	cases := []struct {
		scanMe        []byte
		keywords      []string
		expect        string
		expectErrCode string
	}{
		{ // happy path
			[]byte(`{"foo":{}}`),
			[]string{"foo", "bar"},
			"foo", "",
		},
		{ // length edgecases
			[]byte(``),
			[]string{"foo", "bar"},
			"", wfapi.ECodeSerialization,
		},
		{ // bad parse (unmatched string delims)
			[]byte(`"`),
			[]string{"foo", "bar"},
			"", wfapi.ECodeSerialization,
		},
		{ // empty string edgecase
			[]byte(`""`),
			[]string{"foo", "bar"},
			"", "",
		},
		{ // no match
			[]byte(`"x"`),
			[]string{"foo", "bar"},
			"", "",
		},
		{ // no match but it does match prefix
			[]byte(`"foobar"`),
			[]string{"foo", "bar"},
			"", "",
		},
	}
	for _, c := range cases {
		keyword, err := GuessDocumentType(c.scanMe, c.keywords)
		qt.Check(t, keyword, qt.Equals, c.expect)
		qt.Check(t, serum.Code(err), qt.Equals, c.expectErrCode)
	}
}
