package larkdemo

import (
	"fmt"
	"testing"

	"go.starlark.net/starlark"
)

func TestHello(t *testing.T) {
	// Execute Starlark program in a file.
	thread := &starlark.Thread{Name: "thethreadname"}
	globals, err := starlark.ExecFile(thread, "thefilename.star", `
def fibonacci(n):
	res = list(range(n))
	for i in res[2:]:
		res[i] = res[i-2] + res[i-1]
	return res
`, nil)
	if err != nil {
		panic(err)
	}

	// Retrieve a module global.
	fibonacci := globals["fibonacci"]

	// Call Starlark function from Go.
	v, err := starlark.Call(thread, fibonacci, starlark.Tuple{starlark.MakeInt(10)}, nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("fibonacci(10) = %v\n", v)
}
