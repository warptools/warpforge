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
