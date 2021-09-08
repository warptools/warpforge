/*
	datalark makes IPLD data legible to, and constructable in, starlark.

	Given an IPLD Schema (and optionally, a list of types to focus on),
	datalark can generate a set of starlark constructor functions for those types.
	These functions should generally DWIM ("do what I mean"):
	for structs, they accept kwargs corresponding to the field names, etc.
	Some functions get clever: for example, for structs with stringy representations (stringjoin, etc),
	the representation form can be used as an argument to the constructor instead of the kwargs form,
	and the construction will "DWIM" with that information and parse it in the appropriate way.

	Standard datamodel data is also always legible,
	and a set of functions for creating it can also be obtained from the datalark package.

	All IPLD data exposed to starlark always acts as if it is "frozen", in starlark parlance.
	This should be unsurprising, since IPLD is already oriented around immutability.

	datalark can be used on natural golang structs by combining it with the
	go-ipld-prime/node/bindnode package.
	This may make it an interesting alternative to github.com/starlight-go/starlight
	(although admittedly more complicated; it's probably only worth it if you
	also already value some of the features of IPLD Schemas).

	Another way of using datalark actually allows providing a function to starlark
	which will accept an IPLD Schema and a type name as parameters,
	and will return a constructor for that type.
	(Not yet implemented.)
*/
package datalark

import (
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/warpfork/warpforge/larkdemo/datalark/engine"
	"go.starlark.net/starlark"
)

// InjectGlobals mutates a starlark.StringDict to contain the values in the given Object.
//
// Use this if you want to add things to your global starlark environment without a namespacing element.
// (If you want things *with* a namespacing element, it's sufficient to just put the Object in the StringDict under whatever key you like.)
//
// This is meant to be used with objects like those from ObjOfConstructorsForPrimitives and ObjOfConstructorsForPrototypes.
// It will panic if keys that aren't starlark.String are encountered, if iterators error, etc.
func InjectGlobals(globals starlark.StringDict, obj *datalarkengine.Object) {
	// Technically this would work on any 'starlark.IterableMapping', but I don't think that makes the function more useful, and would make it *less* self-documenting.
	itr := obj.Iterate()
	defer itr.Done()
	var k starlark.Value
	for itr.Next(&k) {
		v, _, err := obj.Get(k)
		if err != nil {
			panic(err)
		}
		globals[string(k.(starlark.String))] = v
	}
}

// ObjOfConstructorsForPrimitives  returns an Object containing constructor functions
// for all the IPLD Data Model kinds -- strings, maps, etc -- as those names, in TitleCase.
//
// An "Object" is like a starlark.Dict but you can also access its members using dotted notation.
// It's a convenient namespacing helper.
// You can either add it as a value to the globals starlark.StringDict and use the key you add it to as a namespace;
// or, you can use the InjectGlobals function to make all the functions available as globals without namespacing.
func ObjOfConstructorsForPrimitives() *datalarkengine.Object {
	obj := datalarkengine.NewObject(7)
	obj.SetKey(starlark.String("Map"), &datalarkengine.Prototype{basicnode.Prototype.Map})
	obj.SetKey(starlark.String("List"), &datalarkengine.Prototype{basicnode.Prototype.List})
	obj.SetKey(starlark.String("Bool"), &datalarkengine.Prototype{basicnode.Prototype.Bool})
	obj.SetKey(starlark.String("Int"), &datalarkengine.Prototype{basicnode.Prototype.Int})
	obj.SetKey(starlark.String("Float"), &datalarkengine.Prototype{basicnode.Prototype.Float})
	obj.SetKey(starlark.String("String"), &datalarkengine.Prototype{basicnode.Prototype.String})
	obj.SetKey(starlark.String("Bytes"), &datalarkengine.Prototype{basicnode.Prototype.Bytes})
	obj.Freeze()
	return obj
}

// ObjOfConstructorsForPrototypes returns an Object containing constructor functions for IPLD typed nodes,
// based on the list of schema.TypedPrototype you provide,
// and using the names of each of those prototype's types as the keys.
//
// An "Object" is like a starlark.Dict but you can also access its members using dotted notation.
// It's a convenient namespacing helper.
// You can either add it as a value to the globals starlark.StringDict and use the key you add it to as a namespace;
// or, you can use the InjectGlobals function to make all the functions available as globals without namespacing.
//
// The reason this function takes `schema.TypedPrototype` as an argument,
// rather than `schema.Type`, is because prototypes contain information about how to actually construct values.
// (A `schema.Type` value only describes the shape of data, but doesn't say how we want to work with it in memory,
// so it's not enough information to create constructor functions out of.)
func ObjOfConstructorsForPrototypes(prototypes []schema.TypedPrototype) *datalarkengine.Object {
	obj := datalarkengine.NewObject(len(prototypes))
	for _, npt := range prototypes {
		obj.SetKey(starlark.String(npt.Type().Name()), &datalarkengine.Prototype{npt})
	}
	obj.Freeze()
	return obj
}
