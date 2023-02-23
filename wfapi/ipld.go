package wfapi

import (
	"fmt"

	"embed"

	_ "github.com/ipld/go-ipld-prime/codec/json" // side-effecting import; registers a codec.
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/schema"
	schemadmt "github.com/ipld/go-ipld-prime/schema/dmt"
	schemadsl "github.com/ipld/go-ipld-prime/schema/dsl"
)

// This file is for IPLD-related helpers and constants.
// (For example, the linksystem: that's legitimately a global, because it's just for plugin config.)

var LinkSystem = cidlink.DefaultLinkSystem()

// TypeSystem describes all our API data types and their representation strategies in IPLD Schema form.
// This is parsed from the wfapi.ipldsch file, which is embedded into the binary at build time.

// embed the wfapi ipld schema from file
//go:embed wfapi.ipldsch
var schFs embed.FS

// Export both the parsed DMT of the schema,
// and the compiled TypeSystem.
//
// (The DMT form is used by other packages that "extend" this schema,
// so that they don't have to do the whole DSL parse again.)
var SchemaDMT, TypeSystem = func() (*schemadmt.Schema, *schema.TypeSystem) {
	r, err := schFs.Open("wfapi.ipldsch")
	if err != nil {
		panic(fmt.Sprintf("failed to open embedded wfapi.ipldsch: %s", err))
	}
	schemaDmt, err := schemadsl.Parse("wfapi.ipldsch", r)
	if err != nil {
		panic(fmt.Sprintf("failed to parse api schema: %s", err))
	}
	ts := new(schema.TypeSystem)
	ts.Init()
	if err := schemadmt.Compile(ts, schemaDmt); err != nil {
		panic(fmt.Sprintf("failed to compile api schema: %s", err))
	}
	return schemaDmt, ts
}()
