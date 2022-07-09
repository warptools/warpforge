package cataloghtml

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/warpfork/warpforge/pkg/workspace"
)

func TestWhee(t *testing.T) {
	// t.Skip("incomplete")
	homedir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	// This is a very sketchy "live" "test" that assumes you've run `warpforge catalog update` before,
	// and operates (readonly!) on that real data.
	cat_dab, err := workspace.OpenCatalog(os.DirFS("/"), filepath.Join(homedir, ".warpforge/catalogs/warpsys")[1:])
	if err != nil {
		panic(err)
	}
	// Output paths are currently hardcoded and can be seen in the config object below.
	// No actual assertions take place on this; the "test" is manually looking at that output.
	cfg := SiteConfig{
		Ctx:        context.Background(),
		Cat_dab:    cat_dab,
		OutputPath: "/tmp/wf-test-cathtml/",
		URLPrefix:  "/tmp/wf-test-cathtml/",
	}
	os.RemoveAll(cfg.OutputPath)
	if err := cfg.CatalogAndChildrenToHtml(); err != nil {
		panic(err)
	}
}
