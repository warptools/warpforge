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
