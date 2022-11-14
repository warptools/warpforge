package healthcheck

import (
	"context"
	"fmt"

	"github.com/serum-errors/go-serum"

	"github.com/warptools/warpforge/cmd/warpforge/internal/util"
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
	path, err := util.BinPath(c.Name)
	if err != nil {
		return serum.Errorf(CodeRunFailure, "Could not find binary: %w", err)
	}
	return serum.Errorf(CodeRunOkay, "path: %s", path)
}
