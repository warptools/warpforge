package dab

import (
	"path/filepath"
	"strings"

	"github.com/serum-errors/go-serum"

	"github.com/warptools/warpforge/wfapi"
)

type FileType string

const MagicFilename_Formula = "formula.wf"
const (
	FileType_Module  FileType = "module"
	FileType_Plot    FileType = "plot"
	FileType_Formula FileType = "formula"
)

var magicFileTypes = map[string]FileType{
	MagicFilename_Module:  FileType_Module,
	MagicFilename_Plot:    FileType_Plot,
	MagicFilename_Formula: FileType_Formula,
}
var types = map[FileType]struct{}{
	FileType_Module:  {},
	FileType_Plot:    {},
	FileType_Formula: {},
}

// GetFileType returns the file type, which is the file name without extension
// e.g., formula.wf -> formula, module.wf -> module, etc...
//
// Errors:
//
//   - warpforge-error-invalid -- if the file name is not recognized
//
// DEPRECATED: there's almost no situation where `dab.GuessDocumentType`
// and looking at the actual content wouldn't be preferable.
func GetFileType(name string) (FileType, error) {
	base := filepath.Base(name)
	if ft, ok := magicFileTypes[base]; ok {
		return ft, nil
	}
	ft := FileType(strings.TrimSuffix(base, filepath.Ext(base)))
	if _, ok := types[ft]; ok {
		return ft, nil
	}
	return "", serum.Error(wfapi.ECodeInvalid,
		serum.WithMessageTemplate("file {{name | q }} is an unknown file type"),
		serum.WithDetail("name", name),
	)
}
