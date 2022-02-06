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
}

type CatalogLineageEnvelope struct {
	CatalogLineage *CatalogLineage
}

type CatalogLineage struct {
	Name     string
	Metadata struct {
		Keys   []string
		Values map[string]string
	}
	Releases []CatalogRelease
}

func init() {
	TypeSystem.Accumulate(schema.SpawnUnion("CatalogMirrorEnvelope",
		[]schema.TypeName{
			"CatalogMirror",
		},
		schema.SpawnUnionRepresentationKeyed(map[string]schema.TypeName{
			"catalogMirror": "CatalogMirror",
		})))

	TypeSystem.Accumulate(schema.SpawnUnion("CatalogMirror",
		[]schema.TypeName{
			"CatalogMirrorByWare",
			"CatalogMirrorByModule",
		},
		schema.SpawnUnionRepresentationKeyed(map[string]schema.TypeName{
			"byWare":   "CatalogMirrorByWare",
			"byModule": "CatalogMirrorByModule",
		})))

	TypeSystem.Accumulate(schema.SpawnMap("CatalogMirrorByWare", "WareID",
		"List__WarehouseAddr", false))
	TypeSystem.Accumulate(schema.SpawnList("List__WarehouseAddr",
		"WarehouseAddr", false))

	TypeSystem.Accumulate(schema.SpawnMap("CatalogMirrorByModule",
		"ModuleName", "CatalogMirrorsByPacktype", false))
	TypeSystem.Accumulate(schema.SpawnMap("CatalogMirrorsByPacktype",
		"Packtype", "List__WarehouseAddr", false))
}

type CatalogMirrorEnvelope struct {
	CatalogMirror *CatalogMirror
}

type CatalogMirrorByWare struct {
	Keys   []WareID
	Values map[WareID][]WarehouseAddr
}

type CatalogMirrorByModule struct {
	Keys   []ModuleName
	Values map[ModuleName]CatalogMirrorsByPacktype
}

type CatalogMirrorsByPacktype struct {
	Keys   []Packtype
	Values map[Packtype][]WarehouseAddr
}

type CatalogMirror struct {
	ByWare   *CatalogMirrorByWare
	ByModule *CatalogMirrorByModule
}

// NEW CATALOG TYPES
func init() {
	TypeSystem.Accumulate(schema.SpawnMap("Catalog", "ModuleName", "CatalogModule", false))

	TypeSystem.Accumulate(schema.SpawnStruct("CatalogModule",
		[]schema.StructField{
			schema.SpawnStructField("name", "ModuleName", false, false),
			schema.SpawnStructField("releases", "Map__ReleaseName__CatalogRelease", false, false),
			schema.SpawnStructField("metadata", "Map__String__String", false, false),
		},
		schema.SpawnStructRepresentationMap(nil)))

	TypeSystem.Accumulate(schema.SpawnMap("Map__ReleaseName__CatalogRelease", "ReleaseName",
		"CatalogRelease", false))
	TypeSystem.Accumulate(schema.SpawnMap("Map__ItemLabel__WareID", "ItemLabel",
		"WareID", false))

	TypeSystem.Accumulate(schema.SpawnStruct("CatalogRelease",
		[]schema.StructField{
			schema.SpawnStructField("name", "ReleaseName", false, false),
			schema.SpawnStructField("items", "Map__ItemLabel__WareID", false, false),
			schema.SpawnStructField("metadata", "Map__String__String", false, false),
		},
		schema.SpawnStructRepresentationMap(nil)))

}

type Catalog struct {
	Keys   []ModuleName
	Values map[ModuleName]CatalogModule
}

type CatalogModule struct {
	Name     ModuleName
	Releases struct {
		Keys   []ReleaseName
		Values map[ReleaseName]CatalogRelease
	}
	Metadata struct {
		Keys   []string
		Values map[string]string
	}
}

type CatalogRelease struct {
	Name  ReleaseName
	Items struct {
		Keys   []ItemLabel
		Values map[ItemLabel]WareID
	}
	Metadata struct {
		Keys   []string
		Values map[string]string
	}
}
