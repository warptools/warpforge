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
			fmt.Printf(msg)
		},
	}
	_, err := starlark.ExecFile(thread, "thefilename.star", `
print(ConstructString("yo"))
`, starlark.StringDict{
		"ConstructString": starlark.NewBuiltin("ConstructString", ConstructString),
	})
	if err != nil {
		panic(err)
	}

	// Output:
	// string{"yo"}
}
