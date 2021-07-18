package wfapi

import "fmt"

type Error struct {
	Code    string      // Something you should be reasonably able to switch upon programmatically in an API.  Sometimes blank, meaning we've wrapped an unknown error, and the Message string is all you've got.
	Message string      // Complete, preformatted message.  Often duplicates some content that may also be found in the Details.
	Details [][2]string // Key:Value ordered details.  Serializes as map.
	Cause   *Error      // Your option to recurse.  Is `*Error` and not `error` because we still want this to have a predictable, explicit json structure (and be unmarshallable).
}

func (e *Error) Error() string {
	return e.Message
}

func ErrorUnknown(msgTmpl string, cause error) *Error {
	return &Error{
		Code:    "warpforge-error-unknown",
		Message: msgTmpl,
		Cause: func() *Error {
			switch c2 := cause.(type) {
			case *Error:
				return c2
			default:
				return &Error{
					Message: cause.Error(), // if you wanted something more specific, stop using this method.
				}
			}
		}(),
	}
}

func ErrorExecutorFailed(executorEngineName string, cause error) *Error {
	return &Error{
		Code:    "warpforge-error-executor-failed",
		Message: fmt.Sprintf("executor engine failed: the %s engine reported error: %s", executorEngineName, cause),
		Details: [][2]string{
			{"engineName", executorEngineName},
			// ideally we'd have more details here, but honestly, our executors don't give us much clarity most of the time, so... we'll see.
		},
		Cause: &Error{
			Message: cause.Error(),
		},
	}
}
