/*
	datalarkengine contains all the low-level binding logic.

	Perhaps somewhat surprisingly, it includes even wrapper types for the more primitive kinds (like string).
	This is important (rather than just converting them directly to starlark's values)
	because we may want things like IPLD type information (or even just NodePrototype) to be retained,
	as well as sometimes wanting the original pointer to be retained for efficiency reasons.
*/
package datalarkengine

import (
	"fmt"

	"github.com/ipld/go-ipld-prime/datamodel"
	"go.starlark.net/starlark"
)

type Value interface {
	starlark.Value
	Node() datamodel.Node
}

func Wrap(val datamodel.Node) (Value, error) {
	// TODO the usual big honking switch about kinds or typekinds.
	panic("nyi")
}

// Unwrap peeks at a starlark Value and see if it is implemented by one of the wrappers in this package;
// if so, it gets the ipld Node back out and returns that.
// Otherwise, it returns nil.
// (Unwrap does not attempt to coerce other starlark values _into_ ipld Nodes.)
func Unwrap(sval starlark.Value) datamodel.Node {
	switch g := sval.(type) {
	case *Map:
		return g.val
	case *Struct:
		return g.val
	case *String:
		return g.val
	default:
		return nil
	}
}

// Attempt to put the starlark Value into the ipld NodeAssembler.
// If we see it's one of our own wrapped types, yank it back out and use AssignNode.
// If it's a starlark string, take that and use AssignString.
// Other starlark primitives, similarly.
// For starlark recursives, error; that's too much to try to coerce, and almost surely you've made a mistake by asking.
//
// This can't attempt to be nice to foreign/user-defined "types" in starlark, either, unfortunately;
// starlark doesn't have a concept of a data model where you can ask what "kind" something is,
// so if it's not *literally* one of the concrete types from starlark that we can recognize, well, we're outta luck.
func assignish(na datamodel.NodeAssembler, sval starlark.Value) error {
	// Unwrap an existing datamodel value if there is one.
	w := Unwrap(sval)
	if w != nil {
		return na.AssignNode(w)
	}
	// Try any of the starlark primitives we can recognize.
	switch s2 := sval.(type) {
	case starlark.Bool:
		return na.AssignBool(bool(s2))
	case starlark.Int:
		i, err := s2.Int64()
		if err {
			return fmt.Errorf("cannot convert starlark value down into int64")
		}
		return na.AssignInt(i)
	case starlark.Float:
		return na.AssignFloat(float64(s2))
	case starlark.String:
		return na.AssignString(string(s2))
	case starlark.Bytes:
		return na.AssignBytes([]byte(s2))
	}
	// TODO: okay, maybe we should actually detect iterables here and handle calmly too.  Or make variants of this function, at least some of which will.

	// No joy yet?  Okay.  Bail.
	return fmt.Errorf("unwilling to coerce starlark value of type %q into ipld datamodel", sval.Type())
}
