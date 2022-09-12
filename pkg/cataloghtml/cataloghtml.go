package cataloghtml

import (
	"context"
	_ "embed"
	"html/template"
	"os"
	"path"
	"path/filepath"
	"reflect"

	"github.com/warpfork/warpforge/pkg/workspace"
	"github.com/warpfork/warpforge/wfapi"
)

var (
	//go:embed catalogIndex.tmpl.html
	catalogIndexTemplate string

	//go:embed catalogModule.tmpl.html
	catalogModuleTemplate string

	//go:embed catalogRelease.tmpl.html
	catalogReleaseTemplate string

	// FUTURE: consider the use of `embed.FS` and `template.ParseFS()`, if there grow to be many files here.
	// It has slightly less compile-time safety checks on filenames, though.
)

type SiteConfig struct {
	Ctx context.Context

	// Data Access Broker for getting Catalog info.
	// Some functions pass around data in memory,
	// but sometimes those objects just contain CIDs, which we'll need to go load.
	// This has helper functions that do the loading.
	// Arguably should be a parameter, but would end up in almost every single function, so, eh.
	Cat_dab workspace.Catalog

	// A plain string for output path prefix is used because golang still lacks
	// an interface for filesystem *writing* -- io/fs is only reading.  Sigh.
	OutputPath string

	// Set to "/" if you'll be publishing at the root of a subdomain.
	URLPrefix string
}

func (cfg SiteConfig) tfuncs() map[string]interface{} {
	return map[string]interface{}{
		"string": func(x interface{}) string {
			// Very small helper function to stringify things.
			// This is useful for things that are literally typedefs of string but the template package isn't smart enough to be calm about unboxing it.
			// (It also does return something for values of non-string types, but not something very useful.)
			return reflect.ValueOf(x).String()
		},
		"url": func(parts ...string) string {
			return path.Join(append([]string{cfg.URLPrefix}, parts...)...)
		},
	}
}

// CatalogAndChildrenToHtml performs CatalogToHtml, and also
// procedes to invoke the html'ing of all modules within.
//
// Errors:
//
// 	- warpforge-error-io -- in case of errors writing out the new html content.
// 	- warpforge-error-internal -- in case of templating errors.
// 	- warpforge-error-catalog-invalid -- in case the catalog data is invalid.
// 	- warpforge-error-catalog-parse -- in case the catalog data failed to parse entirely.
func (cfg SiteConfig) CatalogAndChildrenToHtml() error {
	if err := cfg.CatalogToHtml(); err != nil {
		return err
	}
	modNames := cfg.Cat_dab.Modules()
	for _, modName := range modNames {
		catMod, err := cfg.Cat_dab.GetModule(wfapi.CatalogRef{modName, "", ""})
		if err != nil {
			return err
		}
		if err := cfg.CatalogModuleAndChildrenToHtml(*catMod); err != nil {
			return err
		}
	}
	return nil
}

// doTemplate does the common bits of making files, processing the template,
// and getting the output where it needs to go.
//
// Errors:
//
// 	- warpforge-error-io -- in case of errors writing out the new html content.
// 	- warpforge-error-internal -- in case of templating errors.
func (cfg SiteConfig) doTemplate(outputPath string, tmpl string, data interface{}) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0775); err != nil {
		return wfapi.ErrorIo("couldn't mkdir during cataloghtml emission", nil, err)
	}
	f, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0664)
	if err != nil {
		return wfapi.ErrorIo("couldn't open file for writing during cataloghtml emission", nil, err)
	}
	defer f.Close()

	t := template.Must(template.New("main").Funcs(cfg.tfuncs()).Parse(tmpl))
	if err := t.Execute(f, data); err != nil {
		return wfapi.ErrorInternal("templating failed", err)
	}
	return nil
}

// CatalogToHtml generates a root page that links to all the modules.
//
// This function has no parameters because it uses the DAB in the SiteConfig entirely.
//
// Errors:
//
// 	- warpforge-error-io -- in case of errors writing out the new html content.
// 	- warpforge-error-internal -- in case of templating errors.
func (cfg SiteConfig) CatalogToHtml() error {
	// Future: It's perhaps a bit odd that this uses the workspace.Catalog object instead of the API object.  We probably haven't hammered out appropriate data access helpers yet.
	return cfg.doTemplate(
		filepath.Join(cfg.OutputPath, "index.html"),
		catalogIndexTemplate,
		cfg.Cat_dab.Modules(),
	)
}

// CatalogModuleAndChildrenToHtml performs CatalogModuleToHtml, and also
// procedes to invoke the html'ing of all releases within.
//
// Errors:
//
// 	- warpforge-error-io -- in case of errors writing out the new html content.
// 	- warpforge-error-internal -- in case of templating errors.
// 	- warpforge-error-catalog-invalid -- in case the catalog data is invalid.
// 	- warpforge-error-catalog-parse -- in case the catalog data failed to parse entirely.
func (cfg SiteConfig) CatalogModuleAndChildrenToHtml(catMod wfapi.CatalogModule) error {
	if err := cfg.CatalogModuleToHtml(catMod); err != nil {
		return err
	}
	for _, releaseName := range catMod.Releases.Keys {
		rel, err := cfg.Cat_dab.GetRelease(wfapi.CatalogRef{catMod.Name, releaseName, ""})
		if err != nil {
			return err
		}
		if err := cfg.ReleaseToHtml(catMod, *rel); err != nil {
			return err
		}
	}
	return nil
}

// CatalogModuleToHtml generates a page for a module which enumerates
// and links to all the releases within it,
// as well as enumerates all the metadata attached to the catalog module.
//
// Errors:
//
// 	- warpforge-error-io -- in case of errors writing out the new html content.
// 	- warpforge-error-internal -- in case of templating errors.
func (cfg SiteConfig) CatalogModuleToHtml(catMod wfapi.CatalogModule) error {
	return cfg.doTemplate(
		filepath.Join(cfg.OutputPath, string(catMod.Name), "_module.html"),
		catalogModuleTemplate,
		catMod,
	)
}

// CatalogModuleToHtml generates a page for a release within a catalog module
// which enumerates all the items within it,
// as well as enumerates all the metadata attached to the release.
//
// Possible but not-yet-implemented future features of this output might include:
// linking better to metadata that references other documents (such as Replays);
// links to neighboring (e.g. forward and previous) releases; etc.
//
// Errors:
//
// 	- warpforge-error-io -- in case of errors writing out the new html content.
// 	- warpforge-error-internal -- in case of templating errors.
func (cfg SiteConfig) ReleaseToHtml(catMod wfapi.CatalogModule, rel wfapi.CatalogRelease) error {
	return cfg.doTemplate(
		filepath.Join(cfg.OutputPath, string(catMod.Name), "_releases", string(rel.ReleaseName)+".html"),
		catalogReleaseTemplate,
		map[string]interface{}{
			"Module":  catMod,
			"Release": rel,
		},
	)
}
