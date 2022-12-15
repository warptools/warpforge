package workspaceapi

import (
	"embed"
	"fmt"

	"github.com/warptools/warpforge/wfapi"

	"github.com/ipld/go-ipld-prime/node/bindnode"

	"github.com/ipld/go-ipld-prime/datamodel"

	"github.com/ipld/go-ipld-prime/schema"
	schemadmt "github.com/ipld/go-ipld-prime/schema/dmt"
	schemadsl "github.com/ipld/go-ipld-prime/schema/dsl"
)

// embed the wfapi ipld schema from file
//go:embed wfwsapi.ipldsch
var schFs embed.FS

var SchemaDMT, TypeSystem = func() (*schemadmt.Schema, *schema.TypeSystem) {
	r, err := schFs.Open("wfwsapi.ipldsch")
	if err != nil {
		panic(fmt.Sprintf("failed to open embedded wfwsapi.ipldsch: %s", err))
	}
	schemaDmt, err := schemadsl.Parse("wfwsapi.ipldsch", r)
	if err != nil {
		panic(fmt.Sprintf("failed to parse api schema: %s", err))
	}
	schemaDmt = concat(wfapi.SchemaDMT, schemaDmt)

	ts := new(schema.TypeSystem)
	ts.Init()
	if err := schemadmt.Compile(ts, schemaDmt); err != nil {
		panic(fmt.Sprintf("failed to compile api schema: %s", err))
	}
	return schemaDmt, ts
}()

// concat returns a new schemadmt that's got the types from both.
//
// This function could probably be hoisted upstream.
func concat(a, b *schemadmt.Schema) *schemadmt.Schema {
	nb := schemadmt.Type.Schema.NewBuilder()
	if err := datamodel.Copy(bindnode.Wrap(a, schemadmt.Type.Schema.Type()), nb); err != nil {
		panic(err)
	}
	if err := datamodel.Copy(bindnode.Wrap(b, schemadmt.Type.Schema.Type()), nb); err != nil {
		panic(err)
	}
	return bindnode.Unwrap(nb.Build()).(*schemadmt.Schema)
}
