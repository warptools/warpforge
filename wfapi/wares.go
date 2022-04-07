package wfapi

import (
	"fmt"
)

type WareID struct {
	Packtype Packtype // f.eks. "tar", "git"
	Hash     string   // what it says on the tin.
}

func (w WareID) String() string {
	return fmt.Sprintf("%s:%s", w.Packtype, w.Hash)
}

type Packtype string

// WarehouseAddr is typically parsed as roughly a URL, but we don't deal with that at the API type level.
type WarehouseAddr string

// Placeholder type.  May need better definition.
type FilterMap struct {
	Keys   []string
	Values map[string]string
}

type Mount struct {
	Mode     MountMode
	HostPath string
}

type MountMode string

const (
	MountMode_Readonly  MountMode = "ro"
	MountMode_Readwrite MountMode = "rw"
	MountMode_Overlay   MountMode = "overlay"
)

type Ingest struct {
	GitIngest *GitIngest
}

type GitIngest struct {
	HostPath string
	Ref      string
}
