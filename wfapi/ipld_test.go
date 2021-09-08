package wfapi

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

// Critical lament with this testing style: this validation doesn't happen before other tests.
// We also couldn't do it during the package init, because of lack of ordering there.
// Uff.  lol.
// The consequence is that if you have an invalid schema, you might hear about it from obscure bindnode errors that should be unreachable for a valid schema.

func TestTypeSystemCompiles(t *testing.T) {
	if errs := TypeSystem.ValidateGraph(); errs != nil {
		qt.Assert(t, errs, qt.IsNil)
	}
}
