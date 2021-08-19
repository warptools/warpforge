package wfapi

import (
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/schema"
)

func init() {
	TypeSystem.Accumulate(schema.SpawnStruct("Formula",
		[]schema.StructField{
			schema.SpawnStructField("inputs", "Map__SandboxPort__FormulaInput", false, false),
			schema.SpawnStructField("action", "Action", false, false),
			schema.SpawnStructField("outputs", "Map__OutputName__GatherDirective", false, false),
		},
		schema.SpawnStructRepresentationMap(nil)))
	TypeSystem.Accumulate(schema.SpawnMap("Map__SandboxPort__FormulaInput",
		"SandboxPort", "FormulaInput", false))
	TypeSystem.Accumulate(schema.SpawnMap("Map__OutputName__GatherDirective",
		"OutputName", "GatherDirective", false))

}

type Formula struct {
	Inputs struct {
		Keys   []SandboxPort
		Values map[SandboxPort]FormulaInput
	}
	Action  Action
	Outputs struct {
		Keys   []OutputName
		Values map[OutputName]GatherDirective
	}
}

func init() {
	TypeSystem.Accumulate(schema.SpawnUnion("SandboxPort",
		[]schema.TypeName{
			"SandboxPath",
			"VariableName",
		},
		schema.SpawnUnionRepresentationStringprefix("", map[string]schema.TypeName{
			"/": "SandboxPath",
			"$": "VariableName",
		})))
	TypeSystem.Accumulate(schema.SpawnString("SandboxPath"))
	TypeSystem.Accumulate(schema.SpawnString("VariableName"))
}

type SandboxPort struct {
	SandboxPath  *SandboxPath
	VariableName *VariableName
}

type SandboxPath string

type VariableName string

func init() {
	TypeSystem.Accumulate(schema.SpawnUnion("FormulaInput",
		[]schema.TypeName{
			"FormulaInputSimple",
			"FormulaInputComplex",
		},
		schema.SpawnUnionRepresentationKinded(map[ipld.Kind]schema.TypeName{
			ipld.Kind_String: "FormulaInputSimple",
			ipld.Kind_Map:    "FormulaInputComplex",
		})))
	TypeSystem.Accumulate(schema.SpawnUnion("FormulaInputSimple",
		[]schema.TypeName{
			"WareID",
			"Mount",
			"String",
		},
		schema.SpawnUnionRepresentationStringprefix("", map[string]schema.TypeName{
			"ware:":    "WareID",
			"mount:":   "Mount",
			"literal:": "String",
		})))
	TypeSystem.Accumulate(schema.SpawnStruct("FormulaInputComplex",
		[]schema.StructField{
			schema.SpawnStructField("basis", "FormulaInputSimple", false, false),
			schema.SpawnStructField("filters", "FilterMap", false, false),
		},
		schema.SpawnStructRepresentationMap(nil)))
}

type FormulaInput struct {
	FormulaInputSimple  *FormulaInputSimple
	FormulaInputComplex *FormulaInputComplex
}

type FormulaInputSimple struct {
	WareID  *WareID
	Mount   *Mount
	Literal *string
}

type FormulaInputComplex struct {
	Basis   FormulaInputSimple
	Filters FilterMap
}

func init() {
	TypeSystem.Accumulate(schema.SpawnString("OutputName"))
	TypeSystem.Accumulate(schema.SpawnStruct("GatherDirective",
		[]schema.StructField{
			schema.SpawnStructField("from", "SandboxPort", false, false),
			schema.SpawnStructField("packtype", "Packtype", true, false),
			schema.SpawnStructField("filters", "FilterMap", true, false),
		},
		schema.SpawnStructRepresentationMap(nil)))
}

type OutputName string

type GatherDirective struct {
	From     SandboxPort
	Packtype *Packtype  // 'optional': should be absent iff SandboxPort is a VariableName.
	Filters  *FilterMap // 'optional': must be absent if SandboxPort is a VariableName.
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
