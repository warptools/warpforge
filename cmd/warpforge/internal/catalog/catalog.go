package catalog

import (
	"context"
	"os"
	"path/filepath"

	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/pkg/tracing"
	"github.com/warptools/warpforge/wfapi"

	"github.com/go-git/go-git/v5"
	"go.opentelemetry.io/otel/trace"
)

const defaultCatalogUrl = "https://github.com/warptools/warpsys-catalog.git"

// InstallDefaultRemoteCatalog creates the default catalog by cloning a remote catalog over network.
// This function will do nothing if the default catalog already exists.
//
// Errors:
//
//    - warpforge-error-git -- Cloning catalog fails
//    - warpforge-error-io -- catalog path exists but is in a strange state
func InstallDefaultRemoteCatalog(ctx context.Context, path string) wfapi.Error {
	log := logging.Ctx(ctx)
	// install our default remote catalog as "default-remote" by cloning from git
	// this will noop if the catalog already exists
	defaultCatalogPath := filepath.Join(path, "warpsys")
	_, err := os.Stat(defaultCatalogPath)
	if !os.IsNotExist(err) {
		if err == nil {
			// a dir exists for this catalog, do nothing
			return nil
		}
		return wfapi.ErrorIo("unknown error with catalog path", defaultCatalogPath, err)
	}

	log.Info("", "installing default catalog to %s...", defaultCatalogPath)

	gitCtx, gitSpan := tracing.Start(ctx, "clone catalog", trace.WithAttributes(tracing.AttrFullExecNameGit, tracing.AttrFullExecOperationGitClone))
	defer gitSpan.End()
	_, err = git.PlainCloneContext(gitCtx, defaultCatalogPath, false, &git.CloneOptions{
		URL: defaultCatalogUrl,
	})
	tracing.EndWithStatus(gitSpan, err)

	log.Info("", "installing default catalog complete")

	if err != nil {
		return wfapi.ErrorGit("Unable to git clone catalog", err)
	}
	return nil
}
