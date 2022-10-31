package cataloghtml

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"

	"github.com/alecthomas/chroma/formatters"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/ipld/go-ipld-prime"
	ipldJson "github.com/ipld/go-ipld-prime/codec/json"

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

	//go:embed catalogReplay.tmpl.html
	catalogReplayTemplate string

	//go:embed css/main.css
	mainCssBody []byte

	//go:embed css/toggle.css
	toggleCssBody []byte

	//go:embed css/tabs.css
	tabsCssBody []byte

	//go:embed js.js
	jsBody []byte

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

	// URL to warehouse to use for download links in generated HTML
	// If nil, download links will be disabled
	DownloadURL *string
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
		"subtract": func(a, b int64) string {
			// Very small helper function to subtract numbers in the templates
			return strconv.FormatInt(a-b, 10)
		},
	}
}

// CatalogAndChildrenToHtml performs CatalogToHtml, and also
// procedes to invoke the html'ing of all modules within.
// Additionally, it does all the other "once" things
// (namely, outputs a copy of the css).
//
// Errors:
//
//   - warpforge-error-io -- in case of errors writing out the new html content.
//   - warpforge-error-internal -- in case of templating errors.
//   - warpforge-error-catalog-invalid -- in case the catalog data is invalid.
//   - warpforge-error-catalog-parse -- in case the catalog data failed to parse entirely.
//   - warpforge-error-serialization -- in case the replay plot serialization fails
func (cfg SiteConfig) CatalogAndChildrenToHtml() error {
	// Emit catalog index.
	if err := cfg.CatalogToHtml(); err != nil {
		return err
	}

	// Emit the "once" stuff.
	path := filepath.Join(cfg.OutputPath, "main.css")
	if err := os.WriteFile(path, mainCssBody, 0644); err != nil {
		return wfapi.ErrorIo("couldn't open file for css as part of cataloghtml emission", path, err)
	}

	path = filepath.Join(cfg.OutputPath, "toggle.css")
	if err := os.WriteFile(path, toggleCssBody, 0644); err != nil {
		return wfapi.ErrorIo("couldn't open file for css as part of cataloghtml emission", path, err)
	}

	path = filepath.Join(cfg.OutputPath, "tabs.css")
	if err := os.WriteFile(path, tabsCssBody, 0644); err != nil {
		return wfapi.ErrorIo("couldn't open file for css as part of cataloghtml emission", path, err)
	}

	path = filepath.Join(cfg.OutputPath, "js.js")
	if err := os.WriteFile(path, jsBody, 0644); err != nil {
		return wfapi.ErrorIo("couldn't open file for css as part of cataloghtml emission", path, err)
	}

	// Emit all modules within.
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
//   - warpforge-error-io -- in case of errors writing out the new html content.
//   - warpforge-error-internal -- in case of templating errors.
func (cfg SiteConfig) doTemplate(outputPath string, tmpl string, data interface{}) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0775); err != nil {
		return wfapi.ErrorIo("couldn't mkdir during cataloghtml emission", outputPath, err)
	}
	f, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0664)
	if err != nil {
		return wfapi.ErrorIo("couldn't open file for writing during cataloghtml emission", outputPath, err)
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
//   - warpforge-error-io -- in case of errors writing out the new html content.
//   - warpforge-error-internal -- in case of templating errors.
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
//   - warpforge-error-io -- in case of errors writing out the new html content.
//   - warpforge-error-internal -- in case of templating errors.
//   - warpforge-error-catalog-invalid -- in case the catalog data is invalid.
//   - warpforge-error-catalog-parse -- in case the catalog data failed to parse entirely.
//   - warpforge-error-serialization -- in case the replay plot serialization fails
func (cfg SiteConfig) CatalogModuleAndChildrenToHtml(catMod wfapi.CatalogModule) error {
	if err := cfg.CatalogModuleToHtml(catMod); err != nil {
		return err
	}
	for _, releaseName := range catMod.Releases.Keys {
		ref := wfapi.CatalogRef{catMod.Name, releaseName, ""}
		rel, err := cfg.Cat_dab.GetRelease(ref)
		if err != nil {
			return err
		}
		if err := cfg.ReleaseToHtml(catMod, *rel); err != nil {
			return err
		}
		replay, err := cfg.Cat_dab.GetReplay(ref)
		if err != nil {
			return err
		}
		if replay != nil {
			if err := cfg.ReplayToHtml(catMod, *rel, *replay); err != nil {
				return err
			}
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
//   - warpforge-error-io -- in case of errors writing out the new html content.
//   - warpforge-error-internal -- in case of templating errors.
func (cfg SiteConfig) CatalogModuleToHtml(catMod wfapi.CatalogModule) error {
	return cfg.doTemplate(
		filepath.Join(cfg.OutputPath, string(catMod.Name), "_module.html"),
		catalogModuleTemplate,
		catMod,
	)
}

// ReleaseToHtml generates a page for a release within a catalog module
// which enumerates all the items within it,
// as well as enumerates all the metadata attached to the release.
//
// Possible but not-yet-implemented future features of this output might include:
// linking better to metadata that references other documents (such as Replays);
// links to neighboring (e.g. forward and previous) releases; etc.
//
// Errors:
//
//   - warpforge-error-io -- in case of errors writing out the new html content.
//   - warpforge-error-internal -- in case of templating errors.
func (cfg SiteConfig) ReleaseToHtml(catMod wfapi.CatalogModule, rel wfapi.CatalogRelease) error {
	return cfg.doTemplate(
		filepath.Join(cfg.OutputPath, string(catMod.Name), "_releases", string(rel.ReleaseName)+".html"),
		catalogReleaseTemplate,
		map[string]interface{}{
			"Module":        catMod,
			"Release":       rel,
			"LinkGenerator": downloadLinkGenerator{cfg: cfg},
		},
	)
}

// ReplayToHtml generates a page for a replay within a release.
//
// Errors:
//
//   - warpforge-error-io -- in case of errors writing out the new html content.
//   - warpforge-error-internal -- in case of templating errors.
//   - warpforge-error-serialization -- in case serializing plot fails
func (cfg SiteConfig) ReplayToHtml(catMod wfapi.CatalogModule, rel wfapi.CatalogRelease, replayPlot wfapi.Plot) error {
	plotJson, errRaw := ipld.Marshal(ipldJson.Encode, &replayPlot, wfapi.TypeSystem.TypeByName("Plot"))
	if errRaw != nil {
		return wfapi.ErrorSerialization("failed to serialize module", errRaw)
	}

	return cfg.doTemplate(
		filepath.Join(cfg.OutputPath, string(catMod.Name), "_replays", string(replayPlot.Cid())+".html"),
		catalogReplayTemplate,
		map[string]interface{}{
			"Module":        catMod,
			"Release":       rel,
			"PlotFormatter": plotFormatter{cfg: cfg, json: string(plotJson)},
		},
	)
}

// Helper type to format JSON Plot into HTML with links
type plotFormatter struct {
	cfg  SiteConfig
	json string
}

func (pf plotFormatter) FormattedJson() template.HTML {
	// indent the json
	var indentedJson bytes.Buffer
	err := json.Indent(&indentedJson, []byte(pf.json), "", "  ")
	if err != nil {
		panic("failed to indent json")
	}

	// apply syntax highlighting to json
	lexer := lexers.Get("json")
	style := styles.Get("dracula")
	if err != nil {
		panic(fmt.Sprintf("failed to modify style: %s", err))
	}
	formatter := formatters.Get("html")
	if lexer == nil || style == nil || formatter == nil {
		panic("failed to setup syntax highlighting")
	}
	iterator, err := lexer.Tokenise(nil, indentedJson.String())
	if err != nil {
		panic("failed to tokenize for syntax highlighting")
	}
	var outBuf bytes.Buffer
	err = formatter.Format(&outBuf, style, iterator)
	if err != nil {
		panic("failed to apply syntax highlighting")
	}

	// replace catalog references with links
	// quotations get replaced with their character code (&#34;), so we must
	// use that for replacement
	out := outBuf.String()
	r := regexp.MustCompile(`catalog:([^:]+):([^:]+):([^:&]+)&#34;`)
	prefix := pf.cfg.URLPrefix
	// add trailing slash if needed
	if prefix[len(prefix)-1] != '/' {
		prefix = prefix + "/"
	}
	replaceStr := fmt.Sprintf("<a href=\"%s$1/_releases/$2.html\">catalog:$1:$2:$3</a>&#34;", prefix)
	out = string(r.ReplaceAllString(out, replaceStr))
	return template.HTML(out)
}

type downloadLinkGenerator struct {
	cfg SiteConfig
}

func (dlg downloadLinkGenerator) DownloadLinksAvailable() bool {
	// if download URL prefix is set, a link can be created
	return dlg.cfg.DownloadURL != nil
}

func (dlg downloadLinkGenerator) DownloadUrl(wareId wfapi.WareID) string {
	return fmt.Sprintf("%s/%s/%s/%s", *dlg.cfg.DownloadURL, wareId.Hash[0:3], wareId.Hash[3:6], wareId.Hash)
}
