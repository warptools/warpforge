package wfapi

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/serum-errors/go-serum"
)

const (
	errCodeAlreadyExists = "warpforge-error-already-exists"
	CodeSyscall          = "warpforge-error-syscall"
	CodePlotExecution    = "warpforge-error-plot-execution-failed"
	CodeSerialization    = "warpforge-error-serialization"
	CodePlotInvalid      = "warpforge-error-plot-invalid"
	CodeGeneratorFailed  = "warpforge-error-generator-failed"
	CodeGit              = "warpforge-error-git"
)

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

// ErrorUnknown is returned when an unknown error occurs
//
// Errors:
//
// - warpforge-error-unknown --
func ErrorUnknown(msgTmpl string, cause error) error {
	return serum.Errorf("warpforge-error-unknown", "%s: %w", msgTmpl, cause)
}

// ErrorInternal is for miscellaneous errors that should be handled internally.
// In most cases, prefer to use more specific errors.
// Can be used when an end user is not expected to have viable intervention strategies.
//
// Errors:
//
// - warpforge-error-internal --
func ErrorInternal(msgTmpl string, cause error) error {
	return serum.Errorf("warpforge-error-internal", "%s: %w", msgTmpl, cause)
}

// ErrorSearchingFilesystem is returned when an error occurs during search
//
// Errors:
//
//    - warpforge-error-searching-filesystem --
func ErrorSearchingFilesystem(searchingFor string, cause error) error {
	result := serum.Errorf("warpforge-error-searching-filesystem",
		"error while searching filesystem for %s: %w", searchingFor, cause)
	addDetails(result, [][2]string{
		{"searchingFor", searchingFor},
		// the cause is presumed to have any path(s) relevant.
	})
	return result
}

// ErrorInvalid is returned when something is invalid.
// In most cases, prefer to use more specific errors.`
// The caller must format the message string.
//
// Errors:
//
//  - warpforge-error-invalid --
func ErrorInvalid(message string, deets ...[2]string) error {
	opts := make([]serum.WithConstruction, 0, len(deets))
	for _, d := range deets {
		opts = append(opts, serum.WithDetail(d[0], d[1]))
	}
	opts = append(opts, serum.WithMessageLiteral(message))
	return serum.Error("warpforge-error-invalid", opts...)
}

// ErrorWorkspace is returned when an error occurs when handling a workspace
//
// Errors:
//
//    - warpforge-error-workspace --
func ErrorWorkspace(wsPath string, cause error) error {
	result := serum.Errorf("warpforge-error-workspace",
		"error handling workspace at %q: %w", wsPath, cause)
	addDetails(result, [][2]string{
		{"workspacePath", wsPath},
	})
	return result
}

// ErrorExecutorFailed is returned when a container executor (e.g., runc)
// returns an error.
//
// Errors:
//
//    - warpforge-error-executor-failed --
func ErrorExecutorFailed(executorEngineName string, cause error) error {
	result := serum.Errorf("warpforge-error-executor-failed",
		"executor engine failed: the %s engine reported error: %w", executorEngineName, cause)
	addDetails(result, [][2]string{
		{"engineName", executorEngineName},
		// ideally we'd have more details here, but honestly, our executors don't give us much clarity most of the time, so... we'll see.
	})
	return result
}

// ErrorIo wraps generic I/O errors from the Go stdlib
//
// Errors:
//
//    - warpforge-error-io --
func ErrorIo(context string, path string, cause error) error {
	result := serum.Errorf("warpforge-error-io",
		"io error: %s: %w", context, cause)
	addDetails(result, [][2]string{{"context", context}, {"path", path}})
	return result
}

// ErrorSerialization is returned when a serialization or deserialization error occurs
//
// Errors:
//
//    - warpforge-error-serialization --
func ErrorSerialization(context string, cause error) error {
	result := serum.Errorf(CodeSerialization,
		"serialization error: %s: %w", context, cause)
	addDetails(result, [][2]string{
		{"context", context},
	})
	return result
}

// ErrorWareUnpack is returned when the unpacking of a ware fails
//
// Errors:
//
//    - warpforge-error-ware-unpack --
func ErrorWareUnpack(wareId WareID, cause error) error {
	result := serum.Errorf("warpforge-error-ware-unpack",
		"error unpacking ware %q: %w", wareId, cause)
	addDetails(result, [][2]string{
		{"wareID", wareId.String()},
	})
	return result
}

// ErrorWarePack is returned when the packing of a ware fails
//
// Errors:
//
//    - warpforge-error-ware-pack --
func ErrorWarePack(path string, cause error) error {
	result := serum.Errorf("warpforge-error-ware-pack",
		"error packing ware %q: %w", path, cause)
	addDetails(result, [][2]string{
		{"path", path},
	})
	return result
}

// ErrorWareIdInvalid is returned when a malformed WareID is parsed
//
// Errors:
//
//    - warpforge-error-wareid-invalid --
func ErrorWareIdInvalid(wareId WareID) error {
	return serum.Error("warpforge-error-wareid-invalid",
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
	return serum.Error("warpforge-error-formula-invalid",
		serum.WithMessageTemplate("invalid formula: {{reason}}"),
		serum.WithDetail("reason", reason),
	)
}

// ErrorFormulaExecutionFailed is returned to wrap generic errors that cause
// formula execution to fail.
//
// Errors:
//
//    - warpforge-error-formula-execution-failed --
func ErrorFormulaExecutionFailed(cause error) error {
	return serum.Errorf("warpforge-error-formula-execution-failed",
		"formula execution failed: %w", cause,
	)
}

// ErrorPlotInvalid is returned when a plot contains invalid data
//
// Errors:
//
//    - warpforge-error-plot-invalid --
func ErrorPlotInvalid(reason string) error {
	return serum.Error(CodePlotInvalid,
		serum.WithMessageTemplate("invalid plot: {{reason}}"),
		serum.WithDetail("reason", reason),
	)
}

// ErrorModuleInvalid is returned when a module contains invalid data
//
// Errors:
//
//    - warpforge-error-module-invalid --
func ErrorModuleInvalid(reason string) error {
	return serum.Error("warpforge-error-module-invalid",
		serum.WithMessageTemplate("invalid module: {{reason}}"),
		serum.WithDetail("reason", reason),
	)
}

// ErrorMissingCatalogEntry is returned when a catalog entry cannot be found
//
// Errors:
//
//    - warpforge-error-missing-catalog-entry --
func ErrorMissingCatalogEntry(ref CatalogRef, replayAvailable bool) error {
	var msg string
	var available string
	if replayAvailable {
		msg = fmt.Sprintf("catalog entry %q exists, but content is missing. Re-run recusively to resolve entry.", ref.String())
		available = "true"
	} else {
		msg = fmt.Sprintf("missing catalog entry %q", ref.String())
		available = "false"
	}
	return serum.Error("warpforge-error-missing-catalog-entry",
		serum.WithMessageLiteral(msg),
		serum.WithDetail("catalogRef", ref.String()),
		serum.WithDetail("replayAvailable", available),
	)
}

// ErrorGit is returned when a go-git error occurs
//
// Errors:
//
//    - warpforge-error-git --
func ErrorGit(context string, cause error) error {
	result := serum.Errorf(CodeGit, "git error: %s: %w", context, cause)
	addDetails(result, [][2]string{
		{"context", context},
	})
	return result
}

// ErrorPlotStepFailed is returned execution of a Step within a Plot fails
//
// Errors:
//
//    - warpforge-error-plot-step-failed --
func ErrorPlotStepFailed(stepName StepName, cause error) error {
	result := serum.Errorf("warpforge-error-plot-step-failed", "plot step %q failed: %w", stepName, cause)
	addDetails(result, [][2]string{
		{"stepName", string(stepName)},
	})
	return result
}

// ErrorCatalogParse is returned when parsing of a catalog file fails
//
// Errors:
//
//    - warpforge-error-catalog-parse --
func ErrorCatalogParse(path string, cause error) error {
	result := serum.Errorf("warpforge-error-catalog-parse",
		"parsing of catalog file %q failed: %w", path, cause)
	addDetails(result, [][2]string{
		{"path", path},
	})
	return result
}

// ErrorCatalogInvalid is returned when a catalog contains invalid data
//
// Errors:
//
//    - warpforge-error-catalog-invalid --
func ErrorCatalogInvalid(path string, reason string) error {
	return serum.Error("warpforge-error-catalog-invalid",
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
	return serum.Error(errCodeAlreadyExists,
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
	return serum.Error("warpforge-error-catalog-name",
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
	return serum.Error(errCodeAlreadyExists,
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
	return serum.Error("warpforge-error-missing",
		serum.WithMessageTemplate("file missing at path: {{path|q}}"),
		serum.WithDetail("path", path),
	)
}

// ErrorSyscall is used to wrap errors from the syscall package
//
// Errors:
//
//    - warpforge-error-syscall --
func ErrorSyscall(fmtPattern string, args ...interface{}) error {
	return serum.Errorf(CodeSyscall, fmtPattern, args...)
}

// ErrorPlotExecutionFailed is used to wrap errors around plot execution
// Errors:
//
//    - warpforge-error-plot-execution-failed --
func ErrorPlotExecutionFailed(cause error) error {
	return serum.Errorf(CodePlotExecution, "plot execution failed: %w", cause)
}

// ErrorGeneratorFailed is returned when an external generator fails
//
// Errors:
//
//    - warpforge-error-generator-failed --
func ErrorGeneratorFailed(generatorName string, inputFile string, details string) error {
	return serum.Errorf(CodeGeneratorFailed, "execution of external generator %q for file %q failed: %s", generatorName, inputFile, details)
}

// ErrorDataTooNew is returned when some data was (partially) deserialized,
// but only enough that we could recognize it as being a newer version of message
// than this application supports.
//
// Errors:
//
//    - warpforge-error-datatoonew -- if some data is too new to parse completely.
func ErrorDataTooNew(context string, cause error) error {
	result := serum.Errorf("warpforge-error-datatoonew",
		"while %s, encountered data from an unknown version: %w", context, cause)
	addDetails(result, [][2]string{
		{"context", context},
	})
	return result
}

// addDetails is a helper method to get around the fact that doing a type coercion within
// an expoerted function is not currently allowed by serum.
// We won't need this if serum supports an equivalent to %w in message templates OR
// supports adding details when using serum.Errorf
func addDetails(err error, details [][2]string) {
	s := err.(*serum.ErrorValue)
	s.Data.Details = append(s.Data.Details, details...)
}
