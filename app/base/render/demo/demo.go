package main

import (
	"bytes"
	"fmt"
	"os"

	wfapp "github.com/warptools/warpforge/app"
	"github.com/warptools/warpforge/app/base/render"
)

func main() {
	var buf bytes.Buffer
	wfapp.App.Writer = &buf
	wfapp.App.ErrWriter = &buf
	_ = wfapp.App.Run([]string{"-h"})

	fmt.Println("--------")
	render.Render(buf.Bytes(), os.Stdout, render.Mode_ANSIdown)
	fmt.Println("--------")
}
