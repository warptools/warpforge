package wfapi

import (
	"github.com/ipld/go-ipld-prime/schema"
)

func init() {
	TypeSystem.Accumulate(schema.SpawnUnion("CatalogLineageEnvelope",
		[]schema.TypeName{
			"CatalogLineage",
		},
		schema.SpawnUnionRepresentationKeyed(map[string]schema.TypeName{
			"catalogLineage": "CatalogLineage",
		})))
	TypeSystem.Accumulate(schema.SpawnStruct("CatalogLineage",
		[]schema.StructField{
			schema.SpawnStructField("name", "String", false, false),
			schema.SpawnStructField("metadata", "Map__String__String", false, false),
			schema.SpawnStructField("releases", "List__CatalogRelease", false, false),
		},
		schema.SpawnStructRepresentationMap(nil)))
	TypeSystem.Accumulate(schema.SpawnList("List__CatalogRelease",
		"CatalogRelease", false))
	TypeSystem.Accumulate(schema.SpawnStruct("CatalogRelease",
		[]schema.StructField{
			schema.SpawnStructField("name", "String", false, false),
			schema.SpawnStructField("items", "Map__String__WareID", false, false),
			schema.SpawnStructField("metadata", "Map__String__String", false, false),
		},
		schema.SpawnStructRepresentationMap(nil)))
	TypeSystem.Accumulate(schema.SpawnMap("Map__String__WareID",
		"String", "WareID", false))
	TypeSystem.Accumulate(schema.SpawnMap("Map__String__String",
		"String", "String", false))
}

type CatalogLineageEnvelope struct {
	Index int
	Value interface{}
}

type CatalogLineage struct {
	Name     string
	Metadata map[string]string
	Releases []CatalogRelease
}

type CatalogRelease struct {
	Name     string
	Items    map[string]WareID
	Metadata map[string]string
}
