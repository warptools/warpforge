/*
	datalarkengine contains all the low-level binding logic.

	Perhaps somewhat surprisingly, it includes even wrapper types for the more primitive kinds (like string).
	This is important (rather than just converting them directly to starlark's values)
	because we may want things like IPLD type information (or even just NodePrototype) to be retained,
	as well as sometimes wanting the original pointer to be retained for efficiency reasons.
*/
package datalarkengine

import (
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
