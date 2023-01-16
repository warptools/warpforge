package dab

import (
	"io/fs"
	"strings"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/warptools/warpforge/wfapi"
)

const (
	MagicFilename_Workspace       = ".warpforge"
	MagicFilename_HomeWorkspace   = ".warphome"
	MagicFilename_MirroringConfig = "mirroring.json"
)

// MirroringConfigFromFile loads a wfapi.MirroringConfig from filesystem path.
//
// In typical usage, the filename parameter will have the suffix of MagicFilename_MirroringConfig.
//
// Errors:
//
// 	- warpforge-error-io -- for errors reading from fsys.
// 	- warpforge-error-serialization -- for errors from try to parse the data as a Module.
// 	- warpforge-error-datatoonew -- if encountering unknown data from a newer version of warpforge!
//  - warpforge-error-module-invalid -- when module name is invalid
func MirroringConfigFromFile(fsys fs.FS, filename string) (wfapi.MirroringConfig, error) {
	const situation = "loading a mirroring config"
	if strings.HasPrefix(filename, "/") {
		filename = filename[1:]
	}
	f, err := fs.ReadFile(fsys, filename)
	if err != nil {
		return wfapi.MirroringConfig{}, wfapi.ErrorIo(situation, filename, err)
	}

	mirroringConfig := wfapi.MirroringConfig{}
	_, err = ipld.Unmarshal(f, json.Decode, &mirroringConfig, wfapi.TypeSystem.TypeByName("MirroringConfig"))
	if err != nil {
		return wfapi.MirroringConfig{}, wfapi.ErrorSerialization(situation, err)
	}

	return mirroringConfig, nil
}
