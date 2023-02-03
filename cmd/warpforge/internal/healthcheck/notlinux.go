//go:build !linux

package healthcheck

import "github.com/serum-errors/go-serum"

type utsname struct {
	Sysname    [65]int8
	Nodename   [65]int8
	Release    [65]int8
	Version    [65]int8
	Machine    [65]int8
	Domainname [65]int8
}

func executionAccess(path string) error {
	return serum.Error(CodeRunAmbiguous, serum.WithMessageLiteral("Execution access detection not implemented for non-Linux systems"))
}

func uname() (*utsname, error) {
	return nil, serum.Error(CodeRunAmbiguous, serum.WithMessageLiteral("Kernel info only for Linux systems"))
}
