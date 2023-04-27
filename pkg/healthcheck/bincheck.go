package healthcheck

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/serum-errors/go-serum"

	"github.com/warptools/warpforge/pkg/config"
)

type BinCheck struct {
	Name string
}

func (c *BinCheck) String() string {
	return fmt.Sprintf("Binary Path Check: %q", c.Name)
}

func isExecutable(m fs.FileMode) bool {
	return m&0111 != 0
}

func isSymlink(m fs.FileMode) bool {
	return m&fs.ModeSymlink == fs.ModeSymlink
}

func followLink(path string) string {
	fi, err := os.Lstat(path)
	if err != nil {
		return ""
	}
	for isSymlink(fi.Mode()) {
		path, err = os.Readlink(path)
		if err != nil {
			return ""
		}
		fi, err = os.Lstat(path)
		if err != nil {
			return ""
		}
	}
	return path
}

// Run checks that that an executable can be found for the given executable name
// Errors:
//
//    - warpforge-error-healthcheck-run-okay -- when the binar is found
//    - warpforge-error-healthcheck-run-fail -- when the binary cannot be found
func (c *BinCheck) Run(ctx context.Context) error {
	binPath, err := config.BinPath()
	if err != nil {
		return serum.Error(CodeRunFailure, serum.WithCause(err),
			serum.WithMessageLiteral("Could not find binary"),
		)
	}
	path := filepath.Join(binPath, c.Name)
	fi, err := os.Stat(path)
	if err != nil {
		return serum.Error(CodeRunFailure, serum.WithCause(err),
			serum.WithMessageTemplate("Could not find binary at path {{path|q}}"),
			serum.WithDetail("path", path),
		)
	}
	mode := fi.Mode()
	if !mode.IsRegular() {
		return serum.Error(CodeRunFailure,
			serum.WithMessageTemplate("file {{path|q}} is not a regular file"),
			serum.WithDetail("path", path),
		)
	}
	if !isExecutable(mode) {
		return serum.Error(CodeRunFailure,
			serum.WithMessageTemplate("file {{path|q}} is not executable"),
			serum.WithDetail("path", path),
		)
	}

	if err := executionAccess(path); err != nil {
		return err
	}

	if fi, _ := os.Lstat(path); isSymlink(fi.Mode()) {
		return serum.Errorf(CodeRunOkay, "symlink: %q -> %q", path, followLink(path))
	}

	return serum.Errorf(CodeRunOkay, "path: %s", path)
}
