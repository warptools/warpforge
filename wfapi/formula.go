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
			"SandboxVar",
		},
		schema.SpawnUnionRepresentationStringprefix("", map[string]schema.TypeName{
			"/": "SandboxPath",
			"$": "SandboxVar",
		})))
	TypeSystem.Accumulate(schema.SpawnString("SandboxPath"))
	TypeSystem.Accumulate(schema.SpawnString("SandboxVar"))
}

type SandboxPort struct { // ... dude.  this isn't actually a viable map key.
	// You're gonna need to go back into bindnode and put integer indicators back in.
	// And then, apparently, just make the pointers here... optional.
	SandboxPath *SandboxPath
	SandboxVar  *SandboxVar
}

type SandboxPath string

type SandboxVar string

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

func (fi *FormulaInput) Basis() *FormulaInputSimple {
	switch {
	case fi.FormulaInputSimple != nil:
		return fi.FormulaInputSimple
	case fi.FormulaInputComplex != nil:
		return &fi.FormulaInputComplex.Basis
	default:
		panic("unreachable")
	}
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
	Packtype *Packtype  // 'optional': should be absent iff SandboxPort is a SandboxVar.
	Filters  *FilterMap // 'optional': must be absent if SandboxPort is a SandboxVar.
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
			schema.SpawnStructField("network", "Boolean", true, false),
		},
		schema.SpawnStructRepresentationMap(nil)))
	TypeSystem.Accumulate(schema.SpawnStruct("Action_Script",
		[]schema.StructField{
			schema.SpawnStructField("interpreter", "String", false, false),
			schema.SpawnStructField("contents", "List__String", false, false),
			schema.SpawnStructField("network", "Boolean", true, false),
		},
		schema.SpawnStructRepresentationMap(nil)))
	TypeSystem.Accumulate(schema.SpawnBool("Boolean"))
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
	Network *bool
}
type Action_Script struct {
	Interpreter string
	Contents    []string
	Network     *bool
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

func init() {
	TypeSystem.Accumulate(schema.SpawnStruct("RunRecord",
		[]schema.StructField{
			schema.SpawnStructField("guid", "String", false, false),
			schema.SpawnStructField("time", "Int", false, false),
			schema.SpawnStructField("formulaID", "String", false, false),
			schema.SpawnStructField("exitcode", "Int", false, false),
			schema.SpawnStructField("results", "Map__OutputName__FormulaInputSimple", false, false),
		},
		schema.SpawnStructRepresentationMap(nil)))
	TypeSystem.Accumulate(schema.SpawnMap("Map__OutputName__FormulaInputSimple",
		"OutputName", "FormulaInputSimple", false))
}

type RunRecord struct {
	Guid      string
	Time      int64
	FormulaID string
	Exitcode  int
	Results   struct {
		Keys   []OutputName
		Values map[OutputName]FormulaInputSimple
	}
}
