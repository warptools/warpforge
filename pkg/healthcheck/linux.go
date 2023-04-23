//go:build linux

package healthcheck

import (
	"syscall"

	"github.com/serum-errors/go-serum"
	"golang.org/x/sys/unix"
)

func executionAccess(path string) error {
	err := unix.Access(path, unix.X_OK)
	if err != nil {
		return serum.Error(CodeRunFailure, serum.WithCause(err),
			serum.WithMessageTemplate("warpforge does not have execution access to file {{path|q}}"),
			serum.WithDetail("path", path),
		)
	}
	return nil
}

type utsname syscall.Utsname

func uname() (*utsname, error) {
	var utsname utsname
	err := syscall.Uname((*syscall.Utsname)(&utsname))
	if err != nil {
		return nil, serum.Error(CodeRunFailure, serum.WithCause(err),
			serum.WithMessageLiteral("uname syscall failed: %w"),
		)
	}
	return &utsname, nil
}
