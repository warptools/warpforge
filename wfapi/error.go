package wfapi

import (
	"encoding/json"
	"fmt"
	"os"
)

// Error is a grouping interface for wfapi errors.
// It's also vacuous: there's only one concrete implementation (which is `*ErrorVal`).
// Nonetheless, we use this when declaring return types for functions,
// because it lets us have untyped nils (meaning: avoids the https://golang.org/doc/faq#nil_error problem),
// while also letting us document our functions as having a reasonably confined error type.
type Error interface {
	error
	_wfapiError()
}

// ErrorVal is the one concrete implementation of Error.
// See docs on the Error interface for why the split exists at all.
type ErrorVal struct {
	CodeString string      // Something you should be reasonably able to switch upon programmatically in an API.  Sometimes blank, meaning we've wrapped an unknown error, and the Message string is all you've got.
	Message    string      // Complete, preformatted message.  Often duplicates some content that may also be found in the Details.
	Details    [][2]string // Key:Value ordered details.  Serializes as map.
	Cause      *ErrorVal   // Your option to recurse.  Is `*ErrorVal` and not `error` or `Error` because we still want this to have a predictable, explicit json structure (and be unmarshallable).
}

func (e *ErrorVal) _wfapiError() {}
func (e *ErrorVal) Error() string {
	return e.Message
}
func (e *ErrorVal) Code() string {
	return e.CodeString
}

// wrap takes an unknown error, and if it's *ErrorVal, returns it as such;
// if it's any other golang error, it wraps it in an *ErrorVal which has only the message field set.
//
// This may lose type information (e.g. it's not friendly to `errors.Is`);
// that's a trade we make, because we care about the value being equal to what it will be after a serialization roundtrip.
func wrapErr(err error) *ErrorVal {
	switch c2 := err.(type) {
	case *ErrorVal:
		return c2
	default:
		return &ErrorVal{
			Message: err.Error(),
		}
	}
}

// TerminalError emits an error on stdout as json, and halts immediately.
// In most cases, you should not use this method, and there will be a better place to send errors
// that will be more guaranteed to fit any protocols and scripts better;
// however, this is sometimes used in init methods (where we know no other protocol yet).
func TerminalError(err Error, exitCode int) {
	json.NewEncoder(os.Stdout).Encode(struct {
		Error Error `json:"error"`
	}{err})
	os.Exit(exitCode)
}

// ErrorUnknown is returned when an unknown error occurs
//
// Errors:
//
// - warpforge-error-unknown --
func ErrorUnknown(msgTmpl string, cause error) Error {
	return &ErrorVal{
		CodeString: "warpforge-error-unknown",
		Message:    msgTmpl,
		Cause:      wrapErr(cause),
	}
}

func ErrorSearchingFilesystem(searchingFor string, cause error) Error {
	return &ErrorVal{
		CodeString: "warpforge-error-searching-filesystem",
		Message:    fmt.Sprintf("error while searching filesystem for %s: %s", searchingFor, cause),
		Details: [][2]string{
			{"searchingFor", searchingFor},
			// the cause is presumed to have any path(s) relevant.
		},
		Cause: wrapErr(cause),
	}
}

// ErrorWorkspace is returned when an error occurs when handling a workspace
//
// Errors:
//
//    - warpforge-error-workspace --
func ErrorWorkspace(wsPath string, cause error) Error {
	return &ErrorVal{
		CodeString: "warpforge-error-workspace",
		Message:    fmt.Sprintf("error handling workspace at %q: %s", wsPath, cause),
		Details: [][2]string{
			{"workspacePath", wsPath},
		},
		Cause: wrapErr(cause),
	}
}

// ErrorExecutorFailed is returned when a container executor (e.g., runc)
// returns an error.
//
// Errors:
//
//    - warpforge-error-executor-failed --
func ErrorExecutorFailed(executorEngineName string, cause error) Error {
	return &ErrorVal{
		CodeString: "warpforge-error-executor-failed",
		Message:    fmt.Sprintf("executor engine failed: the %s engine reported error: %s", executorEngineName, cause),
		Details: [][2]string{
			{"engineName", executorEngineName},
			// ideally we'd have more details here, but honestly, our executors don't give us much clarity most of the time, so... we'll see.
		},
		Cause: wrapErr(cause),
	}
}

// ErrorIo wraps generic I/O errors from the Go stdlib
//
// Errors:
//
//    - warpforge-error-io --
func ErrorIo(context string, cause error) Error {
	return &ErrorVal{
		CodeString: "warpforge-error-io",
		Message:    fmt.Sprintf("io error: %s: %s", context, cause),
		Details: [][2]string{
			{"context", context},
		},
		Cause: wrapErr(cause),
	}
}

// ErrorSerialization is returned when a serialization or deserialization error occurs
//
// Errors:
//
//    - warpforge-error-serialization --
func ErrorSerialization(context string, cause error) Error {
	return &ErrorVal{
		CodeString: "warpforge-error-serialization",
		Message:    fmt.Sprintf("serialization error: %s: %s", context, cause),
		Details: [][2]string{
			{"context", context},
		},
		Cause: wrapErr(cause),
	}
}

// ErrorWareUnpack is returned when the unpacking of a ware fails
//
// Errors:
//
//    - warpforge-error-ware-unpack --
func ErrorWareUnpack(wareId WareID, cause error) Error {
	return &ErrorVal{
		CodeString: "warpforge-error-ware-unpack",
		Message:    fmt.Sprintf("error unpacking ware %q: %s", wareId, cause),
		Details: [][2]string{
			{"wareID", wareId.String()},
		},
		Cause: wrapErr(cause),
	}
}

// ErrorWarePack is returned when the packing of a ware fails
//
// Errors:
//
//    - warpforge-error-ware-pack --
func ErrorWarePack(path string, cause error) Error {
	return &ErrorVal{
		CodeString: "warpforge-error-ware-pack",
		Message:    fmt.Sprintf("error packing ware %q: %s", path, cause),
		Details: [][2]string{
			{"path", path},
		},
		Cause: wrapErr(cause),
	}
}

// ErrorFormulaInvalid is returned when a formula contains invalid data
//
// Errors:
//
//    - warpforge-error-formula-invalid --
func ErrorFormulaInvalid(reason string) Error {
	return &ErrorVal{
		CodeString: "warpforge-error-formula-invalid",
		Message:    fmt.Sprintf("invalid formula: %s", reason),
		Details: [][2]string{
			{"reason", reason},
		},
	}
}

// ErrorUnimplemented is returned when a feature is not implemented
//
// Errors:
//
//    - warpforge-error-unimplemented --
func ErrorUnimplemented(feature string) Error {
	return &ErrorVal{
		CodeString: "warpforge-error-unimplemented",
		Message:    fmt.Sprintf("unimplemented: %s", feature),
		Details:    [][2]string{},
	}
}

// ErrorIpld is returned to wrap IPLD errors
//
// Errors:
//
//    - warpforge-error-ipld --
func ErrorIpld(context string, cause error) Error {
	return &ErrorVal{
		CodeString: "warpforge-error-ipld",
		Message:    fmt.Sprintf("ipld error: %s: %s", context, cause),
		Details: [][2]string{
			{"context", context},
		},
		Cause: wrapErr(cause),
	}
}
