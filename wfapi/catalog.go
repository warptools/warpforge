package wfapi

import (
	"fmt"

	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/schema"
)

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

type CatalogReleaseCID string

type Catalog struct {
	Keys   []ModuleName
	Values map[ModuleName]CatalogModule
}

type CatalogModule struct {
	Name     ModuleName
	Releases struct {
		Keys   []ReleaseName
		Values map[ReleaseName]CatalogReleaseCID
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

func (rel *CatalogRelease) Cid() CatalogReleaseCID {
	// convert parsed release to node
	nRelease := bindnode.Wrap(rel, TypeSystem.TypeByName("CatalogRelease"))

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
	return CatalogReleaseCID(lnk.String())
}
