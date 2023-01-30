package healthcheck

import (
	"context"
	"fmt"
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

// Run checks that that an executable can be found for the given executable name
// Errors:
//
//    - warpforge-error-healthcheck-run-okay -- when the binar is found
//    - warpforge-error-healthcheck-run-fail -- when the binary cannot be found
func (c *BinCheck) Run(ctx context.Context) error {
	binPath, err := config.BinPath()
	path := filepath.Join(binPath, c.Name)
	if err != nil {
		return serum.Errorf(CodeRunFailure, "Could not find binary: %w", err)
	}
	return serum.Errorf(CodeRunOkay, "path: %s", path)
}
