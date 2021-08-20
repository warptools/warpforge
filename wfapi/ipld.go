package wfapi

import (
	_ "github.com/ipld/go-ipld-prime/codec/json" // side-effecting import; registers a codec.
	"github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/schema"
)

// This file is for IPLD-related helpers and constants.
// (For example, the linksystem: that's legitimately a global, because it's just for plugin config.)

var LinkSystem = cidlink.DefaultLinkSystem()

// TypeSystem describes all our API data types and their representation strategies in IPLD Schema form.
//
// It's created accumulatively in a bunch of init methods which are near the golang types that are described.
// (This will probably be replaced by something that parses Schema DSL in the future, but for now, this is how we're doing it.)
var TypeSystem = func() *schema.TypeSystem {
	ts := schema.TypeSystem{}
	ts.Init()
	return &ts
}()

func init() {
	// Prelude.
	TypeSystem.Accumulate(schema.SpawnString("String"))
	TypeSystem.Accumulate(schema.SpawnInt("Int"))

	// Common enough (in fact, we wish these were created implicitly):
	TypeSystem.Accumulate(schema.SpawnMap("Map__String__String",
		"String", "String", false))
	TypeSystem.Accumulate(schema.SpawnList("List__String",
		"String", false))
}
