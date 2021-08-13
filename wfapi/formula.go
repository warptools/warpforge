package wfapi

import (
	"github.com/ipld/go-ipld-prime/schema"
)

func init() {
	TypeSystem.Accumulate(schema.SpawnStruct("Formula",
		[]schema.StructField{
			schema.SpawnStructField("inputs", "Map__String__WareID", false, false), // TODO this is oversimplified
			schema.SpawnStructField("action", "Action", false, false),
			schema.SpawnStructField("outputs", "Map__String__String", false, false),
		},
		schema.SpawnStructRepresentationMap(nil)))
}

type Formula struct {
	Inputs struct {
		Keys   []string
		Values map[string]WareID
	}
	Action  Action
	Outputs struct {
		Keys   []string
		Values map[string]string
	}
}

func init() {
	TypeSystem.Accumulate(schema.SpawnUnion("Action",
		[]schema.TypeName{
			"Action_Echo",
			"Action_Exec",
			"Action_Script",
		},
		schema.SpawnUnionRepresentationKeyed(map[string]schema.TypeName{
			"echo":   "Action_Echo",
			"exec":   "Action_Exec",
			"script": "Action_Script",
		})))
	TypeSystem.Accumulate(schema.SpawnStruct("Action_Echo",
		[]schema.StructField{
			// Nothing here.
		},
		schema.SpawnStructRepresentationMap(nil)))
	TypeSystem.Accumulate(schema.SpawnStruct("Action_Exec",
		[]schema.StructField{
			schema.SpawnStructField("command", "List__String", false, false),
		},
		schema.SpawnStructRepresentationMap(nil)))
	TypeSystem.Accumulate(schema.SpawnStruct("Action_Script",
		[]schema.StructField{
			// TODO
		},
		schema.SpawnStructRepresentationMap(nil)))
}

// Action is a union (aka sum type).  Exactly one of its fields will be set.
type Action struct {
	Echo   *Action_Echo
	Exec   *Action_Exec
	Script *Action_Script
}

type Action_Echo struct {
	// Nothing here.  This is just a debug action, and needs no detailed configuration.
}
type Action_Exec struct {
	Command []string
	// TODO
}
type Action_Script struct {
	// TODO
}

func init() {
	TypeSystem.Accumulate(schema.SpawnStruct("FormulaContext",
		[]schema.StructField{
			schema.SpawnStructField("warehouses", "Map__WareID__WarehouseAddr", false, false),
		},
		schema.SpawnStructRepresentationMap(nil)))
	TypeSystem.Accumulate(schema.SpawnStruct("FormulaAndContext",
		[]schema.StructField{
			schema.SpawnStructField("formula", "Formula", false, false),
			schema.SpawnStructField("context", "FormulaContext", true, false),
		},
		schema.SpawnStructRepresentationMap(nil)))
}

type FormulaContext struct {
	Warehouses struct {
		Keys   []WareID
		Values map[WareID]WarehouseAddr
	}
}

type FormulaAndContext struct {
	Formula Formula
	Context *FormulaContext
}
