package helpgen

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/urfave/cli/v2"
)

/*
	As with most files in this package: a word of warning:
	there are mutations to package-scope variables here.

	We updated the default help templates in `urfave/cli` during package init.

	This is technically avoidable (you can set your own values on command objects),
	but there are so many other things that have already forced our hand on pkg vars
	that putting in extra effort to take a high ground here seems quite pointless.
	(The only think we can obtain by leaving the default values alone is a lot of
	boilerplate setting overrides on every single command, and as a bonus getting
	a very-obnoxiously-difficult-to-debug panic from the template engine if you
	were ever to forget to do the override on any single command.  No thanks.)
*/

// helper for heredoc dedenting plus don't do a trailing linebreak.
func docnl(s string) string {
	s = heredoc.Doc(s)
	return s[:len(s)-1]
}

// Appears near the top of each help page.
var helpNameTemplate = docnl(`
	{{$v := offset .HelpName 8}}{{wrap .HelpName 4}}{{if .Usage}} - {{wrap .Usage $v}}{{end}}
`)

// Appears second in each help page.  Should contain short examples.
//
// FUTURE: will be removed, or else needs a drastic rewrite.  The current state of it is almost totally useless.
// A good system should list every flag; this doesn't even try.
// Also, this will get overridden with manual synopsis so often it's unclear if the effort on autogen will be worth it.
//
// FUTURE:CONSIDER: maybe having a new convention like "if ArgsUsage is set, split by lines and prefix each with the command name" would be useful?
var usageTemplate = docnl(`
	{{if .UsageText}}{{wrap .UsageText 4}}{{else}}{{.HelpName}}{{if .VisibleFlags}} [command options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}{{end}}
`)

var authorsTemplate = docnl(`
	{{with $length := len .Authors}}{{if ne 1 $length}}S{{end}}{{end}}:
	    {{range $index, $author := .Authors}}{{if $index}}
	    {{end}}{{$author}}{{end}}
`)

var visibleCommandTemplate = docnl(`

	{{- range .VisibleCommands}}
	### {{join .Names ", "}}
	{{.Usage}}
	{{end}}

`)

// var visibleCommandTemplate = docnl(`
// 	| Subcomand  | Role |
// 	| ---------- | ---- |
// 	{{- range .VisibleCommands}}
// 	| {{join .Names ", "}} | {{.Usage}} |
// 	{{- end}}
// `)

var visibleCommandCategoryTemplate = docnl(`
	{{- range .VisibleCategories}}{{if .Name}}
	    {{.Name}}:{{range .VisibleCommands}}
	        {{join .Names ", "}}{{"\t"}}{{.Usage}}{{end}}{{else}}{{template "visibleCommandTemplate" .}}{{end}}{{end}}
`)

var visibleFlagCategoryTemplate = docnl(`
	{{- range .VisibleFlagCategories}}
	    {{if .Name}}{{.Name}}

	    {{end}}{{$flglen := len .Flags}}{{range $i, $e := .Flags}}{{if eq (subtract $flglen $i) 1}}{{$e}}
	{{else}}{{$e}}
	    {{end}}{{end}}{{end}}
`)

var visibleFlagTemplate = docnl(`
	{{- range $i, $e := .VisibleFlags}}
	{{$e.String}}
	{{end}}
`) // Note the use of `.String` which contains a lot of further complexity.  This stringing is handled via yet another package var -- see references to `FlagStringer`, later in this file.

func init() {
	cli.AppHelpTemplate = appHelpTemplate
	cli.CommandHelpTemplate = commandHelpTemplate
	cli.SubcommandHelpTemplate = subcommandHelpTemplate
}

// commandHelpTemplate is used for just the root command.
var appHelpTemplate = heredoc.Doc(`
	## NAME
	{{template "helpNameTemplate" .}}

	{{- if .UsageText}}
	## USAGE
	{{.UsageText}}
	{{- end}}

	{{- if .Version}}{{if not .HideVersion}}
	## VERSION
	{{.Version}}
	{{- end}}{{end}}

	{{- if .Description}}
	## DESCRIPTION
	{{.Description}}
	{{- end}}

	{{- if len .Authors}}
	## AUTHORS
	{{- template "authorsTemplate" .}}
	{{- end}}

	{{- if .VisibleCommands}}
	## COMMANDS
	{{ printf "" }}
	{{- template "visibleCommandCategoryTemplate" .}}
	{{- end}}

	{{- if .VisibleFlagCategories}}
	## GLOBAL OPTIONS
	{{ printf "" }}
	{{- template "visibleFlagCategoryTemplate" .}}
	{{- else if .VisibleFlags}}
	## GLOBAL OPTIONS
	{{ printf "" }}
	{{- template "visibleFlagTemplate" .}}
	{{- end}}
`)

// commandHelpTemplate is used for a command that has no subcommands.
var commandHelpTemplate = heredoc.Doc(`
	## NAME
	{{template "helpNameTemplate" .}}

	## USAGE
	{{template "usageTemplate" .}}{{if .Category}}

	## CATEGORY
	{{.Category}}{{end}}

	{{- if .Description}}
	## DESCRIPTION
	{{.Description}}
	{{- end}}

	{{- if .VisibleFlagCategories}}
	## OPTIONS
	{{- template "visibleFlagCategoryTemplate" .}}
	{{- else if .VisibleFlags}}
	## OPTIONS
	{{- template "visibleFlagTemplate" .}}
	{{- end}}
`)

// subcommandHelpTemplate is used for a command with more than zero subcommands.
var subcommandHelpTemplate = heredoc.Doc(`
	## NAME
	{{template "helpNameTemplate" .}}

	## USAGE
	{{if .UsageText}}{{wrap .UsageText 4}}{{else}}{{.HelpName}} {{if .VisibleFlags}}command [command options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}{{end}}{{if .Description}}

	## DESCRIPTION
	{{template "descriptionTemplate" .}}{{end}}{{if .VisibleCommands}}

	## COMMANDS
	{{template "visibleCommandTemplate" .}}{{end}}{{if .VisibleFlagCategories}}

	## OPTIONS
	{{template "visibleFlagCategoryTemplate" .}}{{else if .VisibleFlags}}

	## OPTIONS
	{{template "visibleFlagTemplate" .}}{{end}}
`)

//
// And now functions for helping with flags.  There are many of them.
//

func init() {
	cli.FlagStringer = flagStringer
}

func flagStringer(f cli.Flag) string {
	// enforce DocGeneration interface on flags to avoid reflection
	df := f.(cli.DocGenerationFlag)

	placeholder, usage := unquoteUsage(df.GetUsage())
	needsPlaceholder := df.TakesValue()
	if needsPlaceholder && placeholder == "" {
		placeholder = "VALUE"
	}

	// Set default text for all flags except bool flags.
	// For bool flags, display default text, as long as DisableDefaultText is not set.
	defaultValueString := ""
	if bf, ok := f.(*cli.BoolFlag); !ok || !bf.DisableDefaultText {
		if s := df.GetDefaultText(); s != "" {
			defaultValueString = fmt.Sprintf("\n\n(default: **%s**)", s)
		}
	}

	usageWithDefault := strings.TrimSpace(usage + defaultValueString)

	pn := prefixedNames(df.Names(), placeholder)
	sliceFlag, ok := f.(cli.DocGenerationSliceFlag)
	if ok && sliceFlag.IsSliceFlag() {
		pn = pn + " [ " + pn + " ]"
	}

	return withEnvHint(df.GetEnvVars(), fmt.Sprintf("#### %s\n\n%s\n", pn, usageWithDefault))
}

// Returns the placeholder, if any, and the unquoted usage string.
func unquoteUsage(usage string) (string, string) {
	for i := 0; i < len(usage); i++ {
		if usage[i] == '`' {
			for j := i + 1; j < len(usage); j++ {
				if usage[j] == '`' {
					name := usage[i+1 : j]
					usage = usage[:i] + name + usage[j+1:]
					return name, usage
				}
			}
			break
		}
	}
	return "", usage
}

func prefixedNames(names []string, placeholder string) string {
	var prefixed string
	for i, name := range names {
		if name == "" {
			continue
		}

		prefixed += prefixFor(name) + name
		if placeholder != "" {
			prefixed += "=<" + placeholder + ">"
		}
		if i < len(names)-1 {
			prefixed += ", "
		}
	}
	return prefixed
}

func prefixFor(name string) (prefix string) {
	if len(name) == 1 {
		prefix = "-"
	} else {
		prefix = "--"
	}
	return
}

func withEnvHint(envVars []string, str string) string {
	return str + envFormat(envVars, "$", ", $", "")
}

func envFormat(envVars []string, prefix, sep, suffix string) string {
	if len(envVars) > 0 {
		return fmt.Sprintf("\n(env var: %s**%s**%s)", prefix, strings.Join(envVars, sep), suffix)
	}
	return ""
}
