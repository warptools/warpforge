package dab

import (
	"bytes"

	"github.com/serum-errors/go-serum"

	"github.com/warptools/warpforge/wfapi"
)

// GuessDocumentType peeks into some serial data for the first string token,
// and looks to see if matches one of the keywords, and returns that string.
//
// This is useful if you want to figure out which kind of document a file contains,
// and they are using some sort of reasonable capsule types so that they resemble:
//
//   {"foo":{...}}
//     or
//   {"bar.v1":{...}}
//     or
//   {"bar.v2":{...}}
//
// (This is roughly the data format that an IPLD union type with keyed representation will produce,
// and that is the usual way we demarcate things in all of the warpforge APIs.)
//
// If one of the keywords is found, that string and nil are returned.
// If the document fails to be parsed in some way, empty string and an error are returned.
// If the document seemed to be parsable, but none of the keywords are found,
// empty string and a nil error are returned.
//
// This function is restricted to working on JSON,
// and also on strings that don't require quoting or escaping.
// See implementation notes in comments in the function body for possible future work.
//
// A typically effective way to get the byte slice for scanning is by use
// of bufio.NewReader followed by Peek on that value.
// (This leaves that reader overall still ready to feed into a fuller decoder.)
//
// Errors:
//
//   - warpforge-error-serialization -- if the document is complete unparsable.
//   - warpforge-error-serialization -- if none of the keywords (or, no strings at all)
//      can be found within the scan zone.
func GuessDocumentType(scanMe []byte, keywords []string) (string, error) {
	// This code takes considerable shortcuts, and is specific to JSON and limited domains of data.
	//
	// Ideally, feature would use codecs, and take some kind of codec system as a parameter.
	// However, a good codec API to accomplish this task efficiently must support token stepping,
	// because we want to return after a few tokens rather than the whole stream
	// (and perhaps to be really optimal, it would be nice to even be able to return a buffer of the tokens and a codec that can be resumed).
	// Regrettably, `codec ipld.Decoder` does not expose support for token stepping;
	// and though the refmt code they're based on does support token stepping, that's polevaulting quite a few layers
	// (and would not be able to replay any buffered tokens into the ipld.Decoder interface again, either).
	//
	// Since we can't get it done within a single consistent abstraction level with any of the APIs we're currently holding for talking about codecs,
	// instead, this code currently punts in the most intense possible way:
	// it assumes json, and then some specific simplifications that can be made when looking for the first string in a json document.
	//
	// Yes: I'm talking about looking for the first index of a quote character.
	//
	// Then we look for the next quote character and also blithely assume escaping is not relevant.
	// (If your search keywords don't contain such content, then this shakes out to correct.)
	a := bytes.Index(scanMe, []byte{'"'})
	if a < 0 {
		return "", serum.Errorf(wfapi.ECodeSerialization, "cannot detect file type (no markers found within first %d bytes of file)", len(scanMe))
	}
	a++
	b := bytes.Index(scanMe[a:], []byte{'"'})
	if b < 0 {
		return "", serum.Errorf(wfapi.ECodeSerialization, "cannot detect file type (no markers found within first %d bytes of file)", len(scanMe))
	}
	firstString := string(scanMe[a : a+b])
	for _, s := range keywords {
		if s == firstString {
			return firstString, nil
		}
	}
	return "", nil

}
