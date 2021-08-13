package wfapi

import (
	"github.com/ipld/go-ipld-prime/schema"
)

func init() {
	TypeSystem.Accumulate(schema.SpawnStruct("WareID",
		[]schema.StructField{
			schema.SpawnStructField("packtype", "String", false, false),
			schema.SpawnStructField("hash", "String", false, false),
		},
		schema.SpawnStructRepresentationStringjoin(":")))
	TypeSystem.Accumulate(schema.SpawnMap("Map__String__WareID",
		"String", "WareID", false))
}

type WareID struct {
	Packtype string // f.eks. "tar", "git"
	Hash     string // what it says on the tin.
}

func init() {
	TypeSystem.Accumulate(schema.SpawnString("WarehouseAddr"))
	TypeSystem.Accumulate(schema.SpawnMap("Map__WareID__WarehouseAddr",
		"WareID", "WarehouseAddr", false))
}

// WarehouseAddr is typically parsed as roughly a URL, but we don't deal with that at the API type level.
type WarehouseAddr string
