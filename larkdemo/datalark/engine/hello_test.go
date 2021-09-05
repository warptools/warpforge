package datalarkengine

import (
	"fmt"

	"go.starlark.net/starlark"
)

func Example_hello() {
	thread := &starlark.Thread{
		Name: "thethreadname",
		Print: func(thread *starlark.Thread, msg string) {
			//caller := thread.CallFrame(1)
			//fmt.Printf("%s: %s: %s\n", caller.Pos, caller.Name, msg)
			fmt.Printf("%s\n", msg)
		},
	}
	_, err := starlark.ExecFile(thread, "thefilename.star", `
print(ConstructString("yo"))
x = {"bz": "zoo"}

def uwot():
	return {"ay": "zee"}

print(ConstructMap(hey="hai", zonk="wot", **uwot()))
#print(ConstructMap({ConstructString("fun"): "heeey"}))

export = {"uwot": uwot} # i kinda like npm's style for this better than starlark's "every global becomes exported".
# we'd have to see if it's actually viable to wrap the built-in load, too.  i don't see why not, though.
# the naming can become desynchronized, and syntactically mildly redundant, but... i really don't see a problem there.
# see https://github.com/google/starlark-go/blob/master/doc/spec.md#name-binding-and-variables .
#
# apparently you can also use nested function def's to hide things from children, but... uuf?
# i wanna see demos of how to make that not painful and surprising.  and then maybe make our datalark builtins act the same way to avoid weirdness.
#
# ... you can probably just... not worry about any of this, for many months.

`,
		// FUTURE: may want to make a module, interpret it first, and then make it available in the globals...
		//  as a sheer way of getting dotted notation in a consistent way.
		//   We can also probably do this with a map and achieve similar effects, but that seems less cool.
		starlark.StringDict{
			"ConstructString": starlark.NewBuiltin("ConstructString", ConstructString),
			"ConstructMap":    starlark.NewBuiltin("ConstructMap", ConstructMap),
		})
	if err != nil {
		panic(err)
	}

	// Output:
	// string{"yo"}
	// ::()
	// ::[("hey", "hai") ("zonk", "wot") ("ay", "zee")]
	// None
}
