/*
This package contains our custom help text generators,
and wires them into `urfave/cli` at package init time.

We use templates which emit markdown.
Optionally, this can be subsequently post-processed to be
converted to nicer terminal rendering using ANSI codes --
this feature is in another package, so you can disable it.

(The use of package init time is unfortunate,
but package sideeffects cannot be avoided:
package-scope vars are the only option for customizing help processing
that the `urfave/cli` package currently makes available.)
*/
package helpgen

import (
	"io"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/urfave/cli/v2"
)

// printHelpCustom is the entrypoint for `urfave/cli`'s customization.
//
// See the function of the same name upstream for reference.
// This function is considerably derived from it.
func printHelpCustom(out io.Writer, tmpl string, data interface{}, customFuncs map[string]interface{}) {

	const hardwrap = 10000

	funcMap := template.FuncMap{
		"join":           strings.Join,
		"subtract":       subtract,
		"indent":         indent,
		"nindent":        nindent,
		"trim":           strings.TrimSpace,
		"wrap":           func(input string, offset int) string { return wrap(input, offset, hardwrap) },
		"offset":         offset,
		"offsetCommands": offsetCommands,
	}
	for key, value := range customFuncs {
		funcMap[key] = value
	}

	w := tabwriter.NewWriter(out, 1, 8, 4, ' ', 0)
	t := template.Must(template.New("help").Funcs(funcMap).Parse(tmpl))
	template.Must(t.New("helpNameTemplate").Parse(helpNameTemplate))
	template.Must(t.New("usageTemplate").Parse(usageTemplate))
	template.Must(t.New("descriptionTemplate").Parse(descriptionTemplate))
	template.Must(t.New("visibleCommandTemplate").Parse(visibleCommandTemplate))
	template.Must(t.New("versionTemplate").Parse(versionTemplate))
	template.Must(t.New("visibleFlagCategoryTemplate").Parse(visibleFlagCategoryTemplate))
	template.Must(t.New("visibleFlagTemplate").Parse(visibleFlagTemplate))
	template.Must(t.New("visibleGlobalFlagCategoryTemplate").Parse(strings.Replace(visibleFlagCategoryTemplate, "OPTIONS", "GLOBAL OPTIONS", -1)))
	template.Must(t.New("authorsTemplate").Parse(authorsTemplate))
	template.Must(t.New("visibleCommandCategoryTemplate").Parse(visibleCommandCategoryTemplate))

	err := t.Execute(w, data)
	if err != nil {
		panic(err)
	}
	_ = w.Flush()
}

func init() {
	cli.HelpPrinterCustom = printHelpCustom
}
