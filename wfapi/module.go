package wfapi

import (
	"github.com/ipld/go-ipld-prime/schema"
)

func init() {
	TypeSystem.Accumulate(schema.SpawnStruct("Module",
		[]schema.StructField{
			schema.SpawnStructField("plot", "Plot", true, false),
		},
		schema.SpawnStructRepresentationMap(nil)))
}

type Module struct {
	// name might go here?  other config?  unsure honestly, mostly leaving space for future expansion.
	Plot *Plot // Plot is technically considered optional but a module is pretty useless without one.
}
