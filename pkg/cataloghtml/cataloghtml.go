package cataloghtml

import (
	"context"
	"html/template"
	"os"
	"path"
	"path/filepath"
	"reflect"

	"github.com/warpfork/warpforge/pkg/workspace"
	"github.com/warpfork/warpforge/wfapi"
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
		"string": func(x interface{}) string { // golang would you please shut the fuck up and let me be productive, honestly
			// this is for things that are literally typedefs of string but the template package isn't smart enough to be calm about unboxing it.
			return reflect.ValueOf(x).String()
		},
		"url": func(parts ...string) string {
			return path.Join(append([]string{cfg.URLPrefix}, parts...)...)
		},
	}
}

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

// CatalogToHtml generates a root page that links to all the modules.
//
// This function has no parameters because it uses the DAB in the SiteConfig entirely.
func (cfg SiteConfig) CatalogToHtml() error {
	// Future: It's perhaps a bit odd that this uses the workspace.Catalog object instead of the API object.  We probably haven't hammered out appropriate data access helpers yet.
	if err := os.MkdirAll(cfg.OutputPath, 0775); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(cfg.OutputPath, "index.html"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0664)
	if err != nil {
		return err
	}
	defer f.Close()

	// TODO: it's completely bork that we don't have access to the CIDs here.  workspace.Catalog is Not Good right now.
	// TODO: this probably needs sorting to be stable.
	// Future: we should have a CID of the entire catalog tree root snapshot somewhere, too.  (It should probably use prolly trees or something, though, which is not available as a convenient library yet.)
	t := template.Must(template.New("main").Funcs(cfg.tfuncs()).Parse(`
	<html>
	<div style="border: 1px solid; padding 0.5em;">
		<h1 style="display:inline">catalog</h1>
	</div>
	<h2>modules</h2>
	<ul>
	{{- range $moduleName := . }}
		<li><a href="{{ (url (string $moduleName) "index.html") }}">{{ $moduleName }}</a></li>
	{{- end }}
	</ul>
	</html>
	`))
	return t.Execute(f, cfg.Cat_dab.Modules())
}

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

func (cfg SiteConfig) CatalogModuleToHtml(catMod wfapi.CatalogModule) error {
	if err := os.MkdirAll(filepath.Join(cfg.OutputPath, string(catMod.Name)), 0775); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(cfg.OutputPath, string(catMod.Name), "index.html"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0664)
	if err != nil {
		return err
	}
	defer f.Close()

	t := template.Must(template.New("main").Funcs(cfg.tfuncs()).Parse(`
	<html>
	<div style="border: 1px solid; padding 0.5em;">
		<i>module:</i>
		<h1 style="display:inline">{{ .Name }}</h1>
	</div>
	(<a href="{{ (url "index.html") }}">back to root</a>)
	<h2>releases</h2>
	<ul>
	{{- $dot := . -}}
	{{- range $releaseKey := .Releases.Keys }}
		<li><a href="{{ (url (string $dot.Name) (string $releaseKey) "index.html") }}">{{ $releaseKey }}</a> <small>(cid: {{ index $dot.Releases.Values $releaseKey }})</small></li>
	{{- end }}
	</ul>
	<h2>metadata</h2>
	{{- range $metadataKey := .Metadata.Keys }}
		<dt>{{ $metadataKey }}</dt><dd>{{ index $dot.Metadata.Values $metadataKey }}</dd>
	{{- end }}
	</html>
	`))
	return t.Execute(f, catMod)
}

func (cfg SiteConfig) ReleaseToHtml(catMod wfapi.CatalogModule, rel wfapi.CatalogRelease) error {
	if err := os.MkdirAll(filepath.Join(cfg.OutputPath, string(catMod.Name), string(rel.ReleaseName)), 0775); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(cfg.OutputPath, string(catMod.Name), string(rel.ReleaseName), "index.html"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0664)
	if err != nil {
		return err
	}
	defer f.Close()

	t := template.Must(template.New("main").Funcs(cfg.tfuncs()).Parse(`
	<html>
	<div style="border: 1px solid; padding 0.5em;">
		<i>module:</i>
		<h1 style="display:inline">{{ .Module.Name }}</h1>
		<i>release:</i>
		<h1 style="display:inline">{{ .Release.ReleaseName }}</h1>
	</div>
	(<a href="{{ (url "index.html") }}">back to root</a>; <a href="{{ (url (string .Module.Name) "index.html") }}">back to module index</a>)
	<h2>items</h2>
	<ul>
	{{- $dot := .Release -}}
	{{- range $itemKey := .Release.Items.Keys }}
		<li>{{ $itemKey }} : {{ index $dot.Items.Values $itemKey }}</li>
	{{- end }}
	</ul>
	<h2>metadata</h2>
	{{- range $metadataKey := .Release.Metadata.Keys }}
		<dt>{{ $metadataKey }}</dt><dd>{{ index $dot.Metadata.Values $metadataKey }}</dd>
	{{- end }}
	</html>
	`))
	return t.Execute(f, map[string]interface{}{
		"Module":  catMod,
		"Release": rel,
	})
}
