package tracing

import "go.opentelemetry.io/otel/attribute"

// Span attribute keys used by warpforge
const (
	AttrKeyWarpforgeErrorCode     = "warpforge.error.code"
	AttrKeyWarpforgeFormulaId     = "warpforge.formula.id"
	AttrKeyWarpforgeIngestHash    = "warpforge.ingest.hash"
	AttrKeyWarpforgeIngestPath    = "warpforge.ingest.path"
	AttrKeyWarpforgeIngestRev     = "warpforge.ingest.rev"
	AttrKeyWarpforgeStepName      = "warpforge.step.name"
	AttrKeyWarpforgeWareId        = "warpforge.ware.id"
	AttrKeyWarpforgeExecName      = "warpforge.exec.name"
	AttrKeyWarpforgeExecOperation = "warpforge.exec.operation"
)

// Attribute values
const (
	AttrValueExecNameRio           = "rio"
	AttrValueExecNameGit           = "git"
	AttrValueExecNameRunc          = "runc"
	AttrValueExecOperationGitClone = "clone"
	AttrValueExecOperationGitLs    = "ls"
)

// Enumerated attributes
var (
	AttrFullExecNameRio           = attribute.String(AttrKeyWarpforgeExecName, AttrValueExecNameRio)
	AttrFullExecNameGit           = attribute.String(AttrKeyWarpforgeExecName, AttrValueExecNameGit)
	AttrFullExecNameRunc          = attribute.String(AttrKeyWarpforgeExecName, AttrValueExecNameRunc)
	AttrFullExecOperationGitClone = attribute.String(AttrKeyWarpforgeExecOperation, AttrValueExecOperationGitClone)
	AttrFullExecOperationGitLs    = attribute.String(AttrKeyWarpforgeExecOperation, AttrValueExecOperationGitLs)
)
