package wfapi

import (
	"fmt"

	"embed"

	"github.com/ipld/go-ipld-prime"
	_ "github.com/ipld/go-ipld-prime/codec/json" // side-effecting import; registers a codec.
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/schema"
)

// This file is for IPLD-related helpers and constants.
// (For example, the linksystem: that's legitimately a global, because it's just for plugin config.)

var LinkSystem = cidlink.DefaultLinkSystem()

// TypeSystem describes all our API data types and their representation strategies in IPLD Schema form.
// This is parsed from the wfapi.ipldsch file, which is embedded into the binary at build time.

// embed the wfapi ipld schema from file
//go:embed wfapi.ipldsch
var schFs embed.FS

var TypeSystem = func() *schema.TypeSystem {
	schReader, err := schFs.Open("wfapi.ipldsch")
	if err != nil {
		panic(fmt.Sprintf("failed to open embedded wfapi.ipldsch: %s", err))
	}
	ts, err := ipld.LoadSchema("warpforge", schReader)
	if err != nil {
		panic(fmt.Sprintf("failed to parse api schema: %s", err))
	}
	return ts
}()
