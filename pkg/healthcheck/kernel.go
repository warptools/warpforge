package healthcheck

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"unicode"
	"unsafe"

	"github.com/serum-errors/go-serum"
)

// TODO: We probably don't care about domainname/hostname data and maybe we should filter that out.

type KernelInfo struct{}

// Run executes the checker
// Errors:
//
//    - warpforge-error-healthcheck-run-fail -- syscall or serialization failure
//    - warpforge-error-healthcheck-run-ambiguous -- returns kernel info
func (k *KernelInfo) Run(ctx context.Context) error {
	u, err := uname()
	if err != nil {
		return err
	}
	s := kernelInfoString(u)
	return serum.Errorf(CodeRunAmbiguous, "%s", s)
}

func (k *KernelInfo) String() string {
	return "Kernel info"
}

func kernelInfoString(u *utsname) string {
	f := strings.Repeat("\t%10s: %s\n", 6)
	f = strings.TrimRightFunc(f, unicode.IsSpace)
	return fmt.Sprintf("\n"+f,
		"Sysname", convertInt8ToString(u.Sysname[:]),
		"Nodename", convertInt8ToString(u.Nodename[:]),
		"Release", convertInt8ToString(u.Release[:]),
		"Version", convertInt8ToString(u.Version[:]),
		"Machine", convertInt8ToString(u.Machine[:]),
		"Domainname", convertInt8ToString(u.Domainname[:]),
	)
}

func convertInt8ToString(x []int8) string {
	b := unsafe.Slice((*byte)(unsafe.Pointer(&x[0])), len(x))
	b = bytes.TrimRight(b, string([]byte{0}))
	return string(b)
}
