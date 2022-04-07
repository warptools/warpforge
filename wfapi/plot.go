package wfapi

import (
	"fmt"

	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/schema"
)

type PlotCID string

type Plot struct {
	Inputs struct {
		Keys   []LocalLabel
		Values map[LocalLabel]PlotInput
	}
	Steps struct {
		Keys   []StepName
		Values map[StepName]Step
	}
	Outputs struct {
		Keys   []LocalLabel
		Values map[LocalLabel]PlotOutput
	}
}

func (plot *Plot) Cid() PlotCID {
	// convert parsed release to node
	nRelease := bindnode.Wrap(plot, TypeSystem.TypeByName("Plot"))

	// compute CID of parsed release data
	lsys := cidlink.DefaultLinkSystem()
	lnk, errRaw := lsys.ComputeLink(cidlink.LinkPrototype{cid.Prefix{
		Version:  1,    // Usually '1'.
		Codec:    0x71, // 0x71 means "dag-cbor" -- See the multicodecs table: https://github.com/multiformats/multicodec/
		MhType:   0x13, // 0x13 means "sha2-512" -- See the multicodecs table: https://github.com/multiformats/multicodec/
		MhLength: 64,   // sha2-512 hash has a 64-byte sum.
	}}, nRelease.(schema.TypedNode).Representation())
	if errRaw != nil {
		// panic! this should never fail unless IPLD is broken
		panic(fmt.Sprintf("Fatal IPLD Error: lsys.ComputeLink failed for CatalogRelease: %s", errRaw))
	}
	return PlotCID(lnk.String())
}

// StepName is for assigning string names to Steps in a Plot.
// StepNames will be part of wiring things together using Pipes.
//
// Must not contain ':' charaters or unprintables or whitespace.
type StepName string

// LocalLabel is for referencing data within a Plot.
// Input data gets assigned a LocalLabel;
// Pipes pull info from a LocalLabel (possibly together with a StepName to scope it);
// when a Step is evaluated (e.g. turned into a Formula, executed, and produces results),
// the results will become identifiable by a LocalLabel (scoped by the StepName).
//
// (LocalLabel and OutputName are essentially the same thing: an OutputName
// gets casted to being considered a LocalLabel when a Formula's results are hoisted
// into the Plot.)
//
// Must not contain ':' charaters or unprintables or whitespace.
type LocalLabel string

type PlotInput struct {
	PlotInputSimple  *PlotInputSimple
	PlotInputComplex *PlotInputComplex
}

func (pi *PlotInput) Basis() *PlotInputSimple {
	switch {
	case pi.PlotInputSimple != nil:
		return pi.PlotInputSimple
	case pi.PlotInputComplex != nil:
		return &pi.PlotInputComplex.Basis
	default:
		panic("unreachable")
	}
}

type PlotInputSimple struct {
	WareID     *WareID
	Mount      *Mount
	Literal    *Literal
	Pipe       *Pipe
	CatalogRef *CatalogRef
	Ingest     *Ingest
}

type PlotInputComplex struct {
	Basis   PlotInputSimple
	Filters FilterMap
}

type PlotOutput struct {
	Pipe *Pipe
}

type Pipe struct {
	StepName StepName
	Label    LocalLabel
}

type Step struct {
	Plot         *Plot
	Protoformula *Protoformula
}

type Protoformula struct {
	Inputs struct {
		Keys   []SandboxPort
		Values map[SandboxPort]PlotInput
	}
	Action  Action
	Outputs struct {
		Keys   []LocalLabel
		Values map[LocalLabel]GatherDirective
	}
}

type ModuleName string
type ReleaseName string
type ItemLabel string

type CatalogRef struct {
	ModuleName  ModuleName
	ReleaseName ReleaseName
	ItemName    ItemLabel
}

func (c *CatalogRef) String() string {
	return fmt.Sprintf("catalog:%s:%s:%s", c.ModuleName, c.ReleaseName, c.ItemName)
}

type PlotResults struct {
	Keys   []LocalLabel
	Values map[LocalLabel]WareID
}

type PlotExecConfig struct {
	Recursive         bool
	FormulaExecConfig FormulaExecConfig
}
