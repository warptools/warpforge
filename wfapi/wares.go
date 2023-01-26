package wfapi

import (
	"fmt"
	"path/filepath"
)

type WareID struct {
	Packtype Packtype // f.eks. "tar", "git"
	Hash     string   // what it says on the tin.
}

func (w WareID) String() string {
	return fmt.Sprintf("%s:%s", w.Packtype, w.Hash)
}

func (w WareID) Subpath() string {
	return filepath.Join(w.Hash[0:3], w.Hash[3:6], w.Hash)
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
	MountMode_Readonly  MountMode = "readonly"
	MountMode_Readwrite MountMode = "readwrite"
	MountMode_Overlay   MountMode = "overlay"
)

type Ingest struct {
	GitIngest *GitIngest
}

type GitIngest struct {
	HostPath string
	Ref      string
}
