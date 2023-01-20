package wfapi

import (
	"encoding/json"
	"os"

	"github.com/serum-errors/go-serum"
)

// Error codes are loosely of the form "<application>-error-[subsystem]-<kind>".
const (
	ECodeArgument               = "warpforge-error-invalid-argument"         // ECodeArgument may be used when invalid arguments are provided to the warpforge command line.
	ECodeAlreadyExists          = "warpforge-error-already-exists"           // ECodeAlreadyExists may be used when _something_ already exists. Specify _what_ when using this code.  Prefer more specific codes.
	ECodeCatalogInvalid         = "warpforge-error-catalog-invalid"          // ECodeCatalogInvalid may be used when a catalog contains invalid data.
	ECodeCatalogMissingEntry    = "warpforge-error-catalog-missing-entry"    // ECodeCatalogMissingEntry may be used when a catalog item cannot be found.
	ECodeCatalogName            = "warpforge-error-catalog-name"             // ECodeCatalogName may be used for invalid catalog names.
	ECodeCatalogParse           = "warpforge-error-catalog-parse"            // ECodeCatalogParse may be used when parsing catalog data fails.
	ECodeDataTooNew             = "warpforge-error-datatoonew"               // ErrorDataTooNew is returned when some data was (partially) deserialized, but only enough that we could recognize it as being a newer version of message than this application supports.
	ECodeExecutorFailed         = "warpforge-error-executor-failed"          // ECodeExecutorFailed wraps executor errors (e.g. runc errors).
	ECodeFormulaExecutionFailed = "warpforge-error-formula-execution-failed" // EcodeFormulaExecutionFailed wraps generic errors that caused formula execution to fail.
	ECodeFormulaInvalid         = "warpforge-error-formula-invalid"          // ECodeFormulaInvalid may be used when a formula contains invalid data.
	ECodeGeneratorFailed        = "warpforge-error-generator-failed"         // ECodeGeneratorFailed may be used when an external plot generator fails.
	ECodeGit                    = "warpforge-error-git"                      // ECodeGit wraps errors from git libraries or execution.
	ECodeInternal               = "warpforge-error-internal"                 // ECodeInternal is used for errors that are internal and cannot be handled by users. Prefer more specific codes.
	ECodeInvalid                = "warpforge-error-invalid"                  // ECodeInvalid is used when something is invalid. Prefer more specific codes.
	ECodeIo                     = "warpforge-error-io"                       // ECodeIo wraps generic io errors.
	ECodeMissing                = "warpforge-error-missing"                  // ECodeMissing wraps errors for missing files.
	ECodeModuleInvalid          = "warpforge-error-module-invalid"           // ECodeModuleInvalid is returned when a module contains invalid data.
	ECodePlotExecution          = "warpforge-error-plot-execution-failed"    // ECodePlotExecution is used to wrap errors around plot execution.
	ECodePlotInvalid            = "warpforge-error-plot-invalid"             // ECodePlotInvalid is returned when a plot contains invalid data.
	ECodePlotStepFailed         = "warpforge-error-plot-step-failed"         // ECodePlotStepFailed is returned execution of a Step within a Plot fails.
	ECodeSearchingFilesystem    = "warpforge-error-searching-filesystem"     // ECodeSearchingFilesystem is used to wrap filesystem searching errors.
	ECodeSerialization          = "warpforge-error-serialization"            // ECodeSerialization is used for wrapping generic serialization or deserialization failures.
	ECodeSyscall                = "warpforge-error-syscall"                  // ECodeSyscall is used to wrap generic syscall errors. Prefer more specific codes.
	ECodeUnknown                = "warpforge-error-unknown"                  // ECodeUnknown is used for unknown errors. Avoid whenever possible.
	ECodeWareIdInvalid          = "warpforge-error-wareid-invalid"           // ECodeWareIdInvalid is used for parsing malformed ware IDs.
	ECodeWarePack               = "warpforge-error-ware-pack"                // ECodeWarePack is used when packing a ware fails.
	ECodeWareUnpack             = "warpforge-error-ware-unpack"              // ECodeWareUnpack is used when unpacking a ware fails.
	ECodeWorkspace              = "warpforge-error-workspace"                // ECodeWorkspace is used when an error occurs handling a workspace.
)

// IsCode reports whether any error in err's chain matches the given code string.
//
// The chain consists of err itself followed by the sequence of errors obtained
// by repeatedly calling serum.Cause which is similar to calling Unwrap.
//
// An error is considered to match the code string if the result of
// serum.Code(err) is equal to the code string.
func IsCode(err error, code string) bool {
	for err != nil {
		if serum.Code(err) == code {
			return true
		}
		err = serum.Cause(err)
	}
	return false
}

// TerminalError emits an error on stdout as json, and halts immediately.
// In most cases, you should not use this method, and there will be a better place to send errors
// that will be more guaranteed to fit any protocols and scripts better;
// however, this is sometimes used in init methods (where we know no other protocol yet).
func TerminalError(err serum.ErrorInterface, exitCode int) {
	json.NewEncoder(os.Stdout).Encode(struct {
		Error serum.ErrorInterface `json:"error"`
	}{err})
	os.Exit(exitCode)
}

// ErrorSearchingFilesystem is returned when an error occurs during search
//
// Errors:
//
//    - warpforge-error-searching-filesystem --
func ErrorSearchingFilesystem(searchingFor string, cause error) error {
	return serum.Error(ECodeSearchingFilesystem, serum.WithCause(cause),
		serum.WithMessageTemplate("error while searching filesystem for {{searchingFor|q}}"),
		serum.WithDetail("searchingFor", searchingFor),
		// the cause is presumed to have any relevant path(s).
	)
}

// ErrorWorkspace is returned when an error occurs when handling a workspace
//
// Errors:
//
//    - warpforge-error-workspace --
func ErrorWorkspace(wsPath string, cause error) error {
	return serum.Error(ECodeWorkspace, serum.WithCause(cause),
		serum.WithMessageTemplate("error handling workspace at {{workspacePath|q}}"),
		serum.WithDetail("workspacePath", wsPath),
	)
}

// ErrorExecutorFailed is returned when a container executor (e.g., runc)
// returns an error.
//
// Errors:
//
//    - warpforge-error-executor-failed --
func ErrorExecutorFailed(executorEngineName string, cause error) error {
	return serum.Error(ECodeExecutorFailed, serum.WithCause(cause),
		serum.WithMessageTemplate("the {{engineName|q}} engine reported error"),
		serum.WithDetail("engineName", executorEngineName),
		// ideally we'd have more details here, but honestly, our executors don't give us much clarity most of the time, so... we'll see.
	)
}

// DEPRECATED: This just adds a degenerate repetition of the error code.
// Some IO errors do not have paths and the path isn't templated into the error.
// Generally, relevant paths are expected to be included in the cause.
// Instead, call serum.Error directly to wrap errors.
//
// ErrorIo wraps generic I/O errors from the Go stdlib
//
// Errors:
//
//    - warpforge-error-io --
func ErrorIo(context string, path string, cause error) error {
	return serum.Error(ECodeIo, serum.WithCause(cause),
		serum.WithMessageTemplate("io error: {{context}}"),
		serum.WithDetail("context", context),
		serum.WithDetail("path", path),
	)
}

// DEPRECATED: This just adds a degenerate repetition of the error code.
// Instead, call serum.Error directly to wrap errors.
//
// ErrorSerialization is returned when a serialization or deserialization error occurs
//
// Errors:
//
//    - warpforge-error-serialization --
func ErrorSerialization(context string, cause error) error {
	return serum.Error(ECodeSerialization, serum.WithCause(cause),
		serum.WithMessageTemplate("serialization error: {{context}}"),
		serum.WithDetail("context", context),
	)
}

// ErrorWareUnpack is returned when the unpacking of a ware fails
//
// Errors:
//
//    - warpforge-error-ware-unpack --
func ErrorWareUnpack(wareId WareID, cause error) error {
	return serum.Error(ECodeWareUnpack, serum.WithCause(cause),
		serum.WithMessageTemplate("unable to unpack ware {{wareID|q}}"),
		serum.WithDetail("wareID", wareId.String()),
	)
}

// ErrorWarePack is returned when the packing of a ware fails
//
// Errors:
//
//    - warpforge-error-ware-pack --
func ErrorWarePack(path string, cause error) error {
	return serum.Error(ECodeWarePack, serum.WithCause(cause),
		serum.WithMessageTemplate("unable to pack ware at path {{path | q}}"),
		serum.WithDetail("path", path),
	)
}

// ErrorWareIdInvalid is returned when a malformed WareID is parsed
//
// Errors:
//
//    - warpforge-error-wareid-invalid --
func ErrorWareIdInvalid(wareId WareID) error {
	return serum.Error(ECodeWareIdInvalid,
		serum.WithMessageTemplate("invalid WareID: {{wareId}}"),
		serum.WithDetail("wareId", wareId.String()),
	)
}

// ErrorFormulaInvalid is returned when a formula contains invalid data
//
// Errors:
//
//    - warpforge-error-formula-invalid --
func ErrorFormulaInvalid(reason string) error {
	return serum.Error(ECodeFormulaInvalid,
		serum.WithMessageTemplate("invalid formula: {{reason}}"),
		serum.WithDetail("reason", reason),
	)
}

// DEPRECATED: This constructor does not add value.
// Instead, call serum.Error directly to wrap errors.
//
// ErrorFormulaExecutionFailed is returned to wrap generic errors that cause
// formula execution to fail.
//
// Errors:
//
//    - warpforge-error-formula-execution-failed --
func ErrorFormulaExecutionFailed(cause error) error {
	return serum.Error(ECodeFormulaExecutionFailed, serum.WithCause(cause),
		serum.WithMessageLiteral("formula execution failed"),
	)
}

// DEPRECATED: message adds no value
// ErrorPlotInvalid is returned when a plot contains invalid data
//
// Errors:
//
//    - warpforge-error-plot-invalid --
func ErrorPlotInvalid(reason string) error {
	return serum.Error(ECodePlotInvalid,
		serum.WithMessageTemplate("invalid plot: {{reason}}"),
		serum.WithDetail("reason", reason),
	)
}

// DEPRECATED: This constructor does not add value.
// Instead, call serum.Error directly to wrap errors.
//
// ErrorModuleInvalid is returned when a module contains invalid data
//
// Errors:
//
//    - warpforge-error-module-invalid --
func ErrorModuleInvalid(reason string) error {
	return serum.Error(ECodeModuleInvalid,
		serum.WithMessageTemplate("invalid module: {{reason}}"),
		serum.WithDetail("reason", reason),
	)
}

// ErrorMissingCatalogEntry is returned when a catalog entry cannot be found
//
// Errors:
//
//    - warpforge-error-catalog-missing-entry --
func ErrorMissingCatalogEntry(ref CatalogRef, replayAvailable bool) error {
	var msg string
	var available string
	if replayAvailable {
		msg = "catalog entry {{catalogRef | q}} exists, but content is missing. Re-run recusively to resolve entry."
		available = "true"
	} else {
		msg = "missing catalog entry {{catalogRef | q}}"
		available = "false"
	}
	return serum.Error(ECodeCatalogMissingEntry,
		serum.WithMessageTemplate(msg),
		serum.WithDetail("catalogRef", ref.String()),
		serum.WithDetail("replayAvailable", available),
	)
}

// DEPRECATED: This constructor does not add value.
// Instead, call serum.Error directly to wrap errors.
//
// ErrorGit is returned when a go-git error occurs
//
// Errors:
//
//    - warpforge-error-git --
func ErrorGit(context string, cause error) error {
	return serum.Error(ECodeGit, serum.WithCause(cause),
		serum.WithMessageLiteral(context),
	)
}

// ErrorPlotStepFailed is returned execution of a Step within a Plot fails
//
// Errors:
//
//    - warpforge-error-plot-step-failed --
func ErrorPlotStepFailed(stepName StepName, cause error) error {
	return serum.Error(ECodePlotStepFailed, serum.WithCause(cause),
		serum.WithMessageTemplate("plot step {{stepName|q}} failed"),
		serum.WithDetail("stepName", string(stepName)),
	)
}

// ErrorCatalogParse is returned when parsing of a catalog file fails
//
// Errors:
//
//    - warpforge-error-catalog-parse --
func ErrorCatalogParse(path string, cause error) error {
	return serum.Error(ECodeCatalogParse, serum.WithCause(cause),
		serum.WithMessageTemplate("parsing of catalog file {{path|q}} failed"),
		serum.WithDetail("path", path),
	)
}

// ErrorCatalogInvalid is returned when a catalog contains invalid data
//
// Errors:
//
//    - warpforge-error-catalog-invalid --
func ErrorCatalogInvalid(path string, reason string) error {
	return serum.Error(ECodeCatalogInvalid,
		serum.WithMessageTemplate("invalid catalog file {{path|q}}: {{reason}}"),
		serum.WithDetail("path", path),
		serum.WithDetail("reason", reason),
	)
}

// ErrorCatalogItemAlreadyExists is returned when trying to add an item that already exists
//
// Errors:
//
//    - warpforge-error-already-exists --
func ErrorCatalogItemAlreadyExists(path string, itemName ItemLabel) error {
	return serum.Error(ECodeAlreadyExists,
		serum.WithMessageTemplate("item {{itemName|q}} already exists in release file {{path|q}}"),
		serum.WithDetail("path", path),
		serum.WithDetail("itemName", string(itemName)),
	)
}

// ErrorCatalogName is returned when a catalog name is invalid
//
// Errors:
//
//    - warpforge-error-catalog-name --
func ErrorCatalogName(name string, reason string) error {
	return serum.Error(ECodeCatalogName,
		serum.WithMessageTemplate("catalog name {{name|q}} is invalid: {{reason}}"),
		serum.WithDetail("name", name),
		serum.WithDetail("reason", reason),
	)
}

// ErrorFileAlreadyExists is used when a file already exists
//
// Errors:
//
//    - warpforge-error-already-exists --
func ErrorFileAlreadyExists(path string) error {
	return serum.Error(ECodeAlreadyExists,
		serum.WithMessageTemplate("file already exists at path: {{path|q}}"),
		serum.WithDetail("path", path),
	)
}

// ErrorFileMissing is used when an expected file does not exist
//
// Errors:
//
//    - warpforge-error-missing --
func ErrorFileMissing(path string) error {
	return serum.Error(ECodeMissing,
		serum.WithMessageTemplate("file missing at path: {{path|q}}"),
		serum.WithDetail("path", path),
	)
}

// DEPRECATED: This constructor does not add value.
// Instead, call serum.Error directly to wrap errors.
//
// ErrorSyscall is used to wrap errors from the syscall package
//
// Errors:
//
//    - warpforge-error-syscall --
func ErrorSyscall(fmtPattern string, args ...interface{}) error {
	return serum.Errorf(ECodeSyscall, fmtPattern, args...)
}

// DEPRECATED: This constructor does not add value.
// Instead, call serum.Error directly to wrap errors.
//
// ErrorPlotExecutionFailed is used to wrap errors around plot execution
// Errors:
//
//    - warpforge-error-plot-execution-failed --
func ErrorPlotExecutionFailed(cause error) error {
	return serum.Error(ECodePlotExecution, serum.WithCause(cause),
		serum.WithMessageLiteral("plot execution failed"),
	)
}

// ErrorGeneratorFailed is returned when an external generator fails
//
// Errors:
//
//    - warpforge-error-generator-failed --
func ErrorGeneratorFailed(generatorName string, inputFile string, context string) error {
	return serum.Error(ECodeGeneratorFailed,
		serum.WithMessageTemplate("execution of external generator {{generator|q}} for file {{inputFile|q}} failed: {{context}}"),
		serum.WithDetail("generator", generatorName),
		serum.WithDetail("inputFile", inputFile),
		serum.WithDetail("context", context),
	)
}

// ErrorDataTooNew is returned when some data was (partially) deserialized,
// but only enough that we could recognize it as being a newer version of message
// than this application supports.
//
// Errors:
//
//    - warpforge-error-datatoonew -- if some data is too new to parse completely.
func ErrorDataTooNew(context string, cause error) error {
	return serum.Error(ECodeDataTooNew, serum.WithCause(cause),
		serum.WithMessageTemplate("while {{context}}, encountered data from an unknown version"),
		serum.WithDetail("context", context),
	)
}
