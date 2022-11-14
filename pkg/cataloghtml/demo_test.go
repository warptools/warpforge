package cataloghtml

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/warptools/warpforge/pkg/workspace"
)

func TestWhee(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	cat_dab, err := workspace.OpenCatalog(os.DirFS("/"), filepath.Join(cwd, "../../.warpforge/catalog")[1:])
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
