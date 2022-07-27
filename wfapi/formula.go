package wfapi

type FormulaCapsule struct {
	Formula *Formula
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

type SandboxPort struct { // ... dude.  this isn't actually a viable map key.
	// You're gonna need to go back into bindnode and put integer indicators back in.
	// And then, apparently, just make the pointers here... optional.
	SandboxPath *SandboxPath
	SandboxVar  *SandboxVar
}

type SandboxPath string

type SandboxVar string

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

type Literal string

type FormulaInputSimple struct {
	WareID  *WareID
	Mount   *Mount
	Literal *Literal
}

type FormulaInputComplex struct {
	Basis   FormulaInputSimple
	Filters FilterMap
}

type OutputName string

type GatherDirective struct {
	From     SandboxPort
	Packtype *Packtype  // 'optional': should be absent iff SandboxPort is a SandboxVar.
	Filters  *FilterMap // 'optional': must be absent if SandboxPort is a SandboxVar.
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

type FormulaContextCapsule struct {
	FormulaContext *FormulaContext
}

type FormulaContext struct {
	Warehouses struct {
		Keys   []WareID
		Values map[WareID]WarehouseAddr
	}
}

type FormulaAndContext struct {
	Formula FormulaCapsule
	Context *FormulaContextCapsule
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

type FormulaExecConfig struct {
	Interactive        bool
	DisableMemoization bool
}
