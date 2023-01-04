package publisher

import (
	"os"
	"testing"

	"github.com/warptools/warpforge/pkg/workspace"
)

func TestPub(t *testing.T) {
	ws, err := workspace.OpenWorkspace(os.DirFS("/"), "var/home/eric")
	if err != nil {
		t.Error(err)
	}

	if ws == nil {
		t.Errorf("no workspace")
	}

	cat, err := ws.OpenCatalog("my-catalog")
	if err != nil {
		t.Error(err)
	}

	err = PublishCatalog(*ws, cat)
	if err != nil {
		t.Error(err)
	}
}
