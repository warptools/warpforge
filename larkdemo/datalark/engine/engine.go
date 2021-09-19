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
	"github.com/ipld/go-ipld-prime/schema"
	"go.starlark.net/starlark"
)

type Value interface {
	starlark.Value
	Node() datamodel.Node
}

func Wrap(n datamodel.Node) (Value, error) {
	if nt, ok := n.(schema.TypedNode); ok {
		switch nt.Type().TypeKind() {
		case schema.TypeKind_Struct:
			return &Struct{n}, nil
		case schema.TypeKind_Union:
			panic("nyi")
		case schema.TypeKind_Enum:
			panic("nyi")
		case schema.TypeKind_Map:
			// fall down to plain data model behavior
		case schema.TypeKind_List:
			// fall down to plain data model behavior
		case schema.TypeKind_Unit:
			// fall down to plain data model behavior
		case schema.TypeKind_Bool:
			// fall down to plain data model behavior
		case schema.TypeKind_Int:
			// fall down to plain data model behavior
		case schema.TypeKind_Float:
			// fall down to plain data model behavior
		case schema.TypeKind_String:
			// fall down to plain data model behavior
		case schema.TypeKind_Bytes:
			// fall down to plain data model behavior
		case schema.TypeKind_Link:
			// fall down to plain data model behavior
		case schema.TypeKind_Invalid:
			panic("uninitialized memory?")
		default:
			panic("unreachable")
		}
	}
	switch n.Kind() {
	case datamodel.Kind_Map:
		return &Map{n}, nil
	case datamodel.Kind_List:
		panic("nyi")
	case datamodel.Kind_Null:
		panic("nyi")
	case datamodel.Kind_Bool:
		panic("nyi")
	case datamodel.Kind_Int:
		panic("nyi")
	case datamodel.Kind_Float:
		panic("nyi")
	case datamodel.Kind_String:
		return &String{n}, nil
	case datamodel.Kind_Bytes:
		panic("nyi")
	case datamodel.Kind_Link:
		panic("nyi")
	case datamodel.Kind_Invalid:
		panic("uninitialized memory?")
	default:
		panic("unreachable")
	}
}

// Unwrap peeks at a starlark Value and see if it is implemented by one of the wrappers in this package;
// if so, it gets the ipld Node back out and returns that.
// Otherwise, it returns nil.
// (Unwrap does not attempt to coerce other starlark values _into_ ipld Nodes.)
func Unwrap(sval starlark.Value) datamodel.Node {
	if g, ok := sval.(Value); ok {
		return g.Node()
	}
	return nil
}

// Attempt to put the starlark Value into the ipld NodeAssembler.
// If we see it's one of our own wrapped types, yank it back out and use AssignNode.
// If it's a starlark string, take that and use AssignString.
// Other starlark primitives, similarly.
// Dicts and lists are also handled.
//
// This makes some attempt to be nice to foreign/user-defined "types" in starlark as well;
// in particular, anything implementing `starlark.IterableMapping` will be converted into map-building assignments,
// and anything implementing just `starlark.Iterable` (and not `starlark.IterableMapping`) will be converted into list-building assignments.
// However, there is no support for primitives unless they're one of the concrete types from the starlark package;
// starlark doesn't have a concept of a data model where you can ask what "kind" something is,
// so if it's not *literally* one of the concrete types that we can match on, well, we're outta luck.
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
	case starlark.IterableMapping:
		size := -1
		if s3, ok := s2.(starlark.Sequence); ok {
			size = s3.Len()
		}
		ma, err := na.BeginMap(int64(size))
		if err != nil {
			return err
		}
		itr := s2.Iterate()
		defer itr.Done()
		var k starlark.Value
		for itr.Next(&k) {
			if err := assignish(ma.AssembleKey(), k); err != nil {
				return err
			}
			v, _, err := s2.Get(k)
			if err != nil {
				return err
			}
			if err := assignish(ma.AssembleValue(), v); err != nil {
				return err
			}
		}
		return ma.Finish()
	case starlark.Iterable:
		size := -1
		if s3, ok := s2.(starlark.Sequence); ok {
			size = s3.Len()
		}
		la, err := na.BeginList(int64(size))
		if err != nil {
			return err
		}
		itr := s2.Iterate()
		defer itr.Done()
		var v starlark.Value
		for itr.Next(&v) {
			if err := assignish(la.AssembleValue(), v); err != nil {
				return err
			}
		}
		return la.Finish()
	}

	// No joy yet?  Okay.  Bail.
	return fmt.Errorf("unwilling to coerce starlark value of type %q into ipld datamodel", sval.Type())
}
