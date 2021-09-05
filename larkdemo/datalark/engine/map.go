package datalarkengine

import (
	"errors"
	"fmt"

	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/printer"
	"github.com/ipld/go-ipld-prime/schema"
	"go.starlark.net/starlark"
)

func ConstructMap(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// TODO grab the type info we bound earlier (somehow) into 'b'?

	// TODO should have several different options:
	//  - accept a single arg of a starlark dict and bounce it in.
	//    - maybe even multiple, and merge them?  maybe.
	//  - accept as many kwargs as you like.
	//  - positional args?  eh, hrm.  maybe.  or maybe not.  i dislike pairs-styled interfaces.
	//  - list of tuples?  does starlark have tuples?  idk.  if this is syntactically obvious, then sure, let's have it.
	// TODO I think there may be some destructing syntax (`**kwargs`...?) that we should understand before going wild with this.
	//  update:
	//    - these are medium powerful.  they do let you unpack a dict.  nice.
	//    - "keyword argument may not follow **kwargs"... so you can't use them to easily override some values.
	//    - this syntax actually allows you to sneak the same key in twice (probably a bug!).
	//    - you can't use kwargs for all strings, so in general, no we can't use kwargs as the exclusive option for any of this.
	//    - yes, you can use `**{"ay":"bee"}` and it works.
	//    - yes, you can use `**uwot()` on a function that returns a dict and it works.
	//    - no, you can not use more than one doublestar in the same function invocaton.
	//    - unknown if you can use doublestar on something that's iterable (or whatever) but not literally a built-in dict.  (probably?)
	//    - tangentally: no, i'm pretty sure you can't add a '+' binary operator on the built-in dict type.

	// User stories we know we'll hit immediately:
	// - kwargs will usually suffice nicely for Plot.steps.  Cool that's easy.
	// - Sometimes we're going to have to parse stringish arguments.  Formula inputs are keyed with unions.  Tricky.  Fun!
	// - Yep, some things have keys that are invalid for kwargs syntax.  Formula inputs, literally all the time, force you to, in fact.

	// var dict *starlark.Dict // really more generally ought to be a Mappable, but these unpack helpers don't support that.  Iterable would sorta work but we don't want lists.
	// if err := starlark.UnpackPositionalArgs("datalark.Map", args, kwargs, 0, &dict); err != nil {
	// 	return nil, err
	// }

	fmt.Printf("::%v\n", args)
	fmt.Printf("::%v\n", kwargs)

	return starlark.None, nil
}

var _ starlark.Mapping = (*Map)(nil)

func WrapMap(val datamodel.Node) (*Map, error) {
	if val.Kind() != datamodel.Kind_Map {
		return nil, fmt.Errorf("WrapMap must be used on a node of kind 'map'!")
	}
	return &Map{val}, nil
}

type Map struct {
	val datamodel.Node
}

func (g *Map) Node() datamodel.Node {
	return g.val
}
func (g *Map) Type() string {
	if tn, ok := g.val.(schema.TypedNode); ok {
		return fmt.Sprintf("datalark_map<%T>", tn.Type().Name())
	}
	return fmt.Sprintf("datalark_map")
}
func (g *Map) String() string {
	return printer.Sprint(g.val)
}
func (g *Map) Freeze() {}
func (g *Map) Truth() starlark.Bool {
	return true
}
func (g *Map) Hash() (uint32, error) {
	return 0, errors.New("TODO")
}

// Get implements part of `starlark.Mapping`.
func (g *Map) Get(in starlark.Value) (out starlark.Value, found bool, err error) {
	if _, ok := in.(Value); ok {
		// TODO: unbox it and use LookupByNode.
	}
	// TODO: coerce to string?  (don't use the String method, it's a printer, not what want.)
	// TODO: it has now become high time to standardize the "not found" errors from the Node API!
	ks := in.String() // FIXME placeholder; objectively and clearly wrong.
	n, err := g.val.LookupByString(ks)
	if err != nil {
		return nil, false, err
	}
	w, err := Wrap(n)
	return w, true, err
}

// TODO: Items?  Keys?  Len?  Iterate?  Attr?  AttrNames?
