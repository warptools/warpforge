package spark

import (
	"context"
	"fmt"
	"strings"

	"github.com/serum-errors/go-serum"

	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/pkg/workspaceapi"
)

type formatter struct {
	Markup
	Style
	color bool
}

func formatPhase(a workspaceapi.ModuleStatusAnswer, err error) Phase {
	if err != nil {
		return Code2Phase(serum.Code(err))
	}
	return Status2Phase[a.Status]
}

func (f formatter) formatStyle(a workspaceapi.ModuleStatusAnswer, err error) string {
	switch f.Style {
	case StylePretty:
		return dasMap[formatPhase(a, err)]
	case StylePhase:
		return string(formatPhase(a, err))
	case StyleApi:
		fallthrough
	default:
		if err != nil {
			return serum.Code(err)
		}
		return string(a.Status)
	}
}
func (f formatter) format(ctx context.Context, a workspaceapi.ModuleStatusAnswer, err error) string {
	logger := logging.Ctx(ctx)
	logger.Debug("", "output formatter: %#v", f)
	raw := f.formatStyle(a, err)
	logger.Debug("", "format raw: %s", raw)
	logger.Debug("", "format phase: %s", formatPhase(a, err))

	switch f.Markup {
	case MarkupBash:
		if f.color {
			return fmt.Sprintf("\\[%s\\]%s\\[%s\\]", dasAnsiColorMap[formatPhase(a, err)], raw, AnsiColorReset)
		}
		return fmt.Sprintf("%s", raw)
	case MarkupAnsi:
		if f.color {
			return fmt.Sprintf("%s%s%s", dasAnsiColorMap[formatPhase(a, err)], raw, AnsiColorReset)
		}
		return fmt.Sprintf("%s", raw)
	case MarkupPango:
		if f.color {
			return fmt.Sprintf("<span %s>%s</span>", dasPangoColorMap[formatPhase(a, err)], raw)
		}
		return fmt.Sprintf("<span>%s</span>", raw)
	case MarkupNone:
		fallthrough
	default:
		return raw
	}
}

type Markup string

const (
	MarkupAnsi  Markup = "ansi"
	MarkupBash  Markup = "bash"
	MarkupNone  Markup = "none"
	MarkupPango Markup = "pango"
)

var MarkupList = []Markup{
	MarkupAnsi,
	MarkupBash,
	MarkupNone,
	MarkupPango,
}

func ValidateMarkup(input string) Markup {
	input = strings.ToLower(input)
	for _, m := range MarkupList {
		if input == string(m) {
			return m
		}
	}
	return DefaultMarkup
}

func ValidateStyle(input string) Style {
	input = strings.ToLower(input)
	for _, s := range StyleList {
		if input == string(s) {
			return s
		}
	}
	return DefaultStyle
}

type Style string

const (
	StyleApi    Style = "api"
	StylePhase  Style = "phase"
	StylePretty Style = "pretty"
)

var StyleList = []Style{
	StyleApi,
	StylePhase,
	StylePretty,
}

const (
	DefaultMarkup = MarkupAnsi
	DefaultStyle  = StylePretty
)

// Phase is a 3-character code for all output conditions.
type Phase string

const (
	Phase_NoModule   Phase = "nop" // spark-side only: couldn't find a module.
	Phase_NoSocket   Phase = "dwn" // spark-side only: no daemon up.
	Phase_Wat        Phase = "wat" // spark-side only: we had comms errors, or daemon sent nonsense.
	Phase_NoPlan     Phase = "non" // daemon does not have plans for this thing.
	Phase_Queued     Phase = "inq" // queued in warpforge.
	Phase_InProgress Phase = "wip" // actively running, like, we're streaming logs out.
	Phase_Rejected   Phase = "rej" // rejected by a warpforge
	Phase_Saving     Phase = "sav" // done, ran user code completely, now saving user outputs. //TODO
	Phase_DoneGood   Phase = "yay" // done, ran user code completely: zero exit.
	Phase_DoneNoGood Phase = "aww" // done, ran user code completely: non-zero exit.
)

var Status2Phase = map[workspaceapi.ModuleStatus]Phase{
	workspaceapi.ModuleStatus_NoInfo:             Phase_NoPlan,
	workspaceapi.ModuleStatus_Queuing:            Phase_Queued,
	workspaceapi.ModuleStatus_InProgress:         Phase_InProgress,
	workspaceapi.ModuleStatus_FailedProvisioning: Phase_Rejected,
	workspaceapi.ModuleStatus_ExecutedSuccess:    Phase_DoneGood,
	workspaceapi.ModuleStatus_ExecutedFailed:     Phase_DoneNoGood,
}

func Code2Phase(code string) Phase {
	switch code {
	case SCodeNoModule:
		return Phase_NoModule
	case SCodeNoSocket:
		return Phase_NoSocket
	case SCodeUnknown:
		fallthrough
	default:
		return Phase_Wat
	}
}

const AnsiColorReset = "\x1B[0m"

//TODO: colorblind coloring map AND/OR allowing custom color maps.
// Probably swap green/red -> blue/yellow respectively?

var dasAnsiColorMap = map[Phase]string{
	Phase_NoModule:   "\x1B[1;90m",                // grey
	Phase_NoSocket:   "\x1B[1;90m",                // grey
	Phase_Wat:        "\x1B[5m\x1B[41m\x1B[1;33m", // blink yellow, red bg
	Phase_NoPlan:     "\x1B[1;90m",                // grey
	Phase_Queued:     "\x1B[33m",                  // brown
	Phase_InProgress: "\x1B[33m",                  // brown
	Phase_Rejected:   "\x1B[31m",                  // red
	Phase_Saving:     "\x1B[33m",                  // brown
	Phase_DoneGood:   "\x1B[32m",                  // green
	Phase_DoneNoGood: "\x1B[1;31m",                // red
}

var dasMap = map[Phase]string{
	Phase_NoModule:   "---",
	Phase_NoSocket:   "-↯-",
	Phase_Wat:        " ! ",
	Phase_NoPlan:     "┐-┌",
	Phase_Queued:     "⟨║⟩",
	Phase_InProgress: "⟨⇋⟩",
	Phase_Rejected:   "═∅═",
	Phase_Saving:     "⟨∴⟩",
	Phase_DoneGood:   "↯↯↯",
	Phase_DoneNoGood: "↯!↯",
}

var dasPangoColorMap = map[Phase]string{
	Phase_NoModule:   `foreground="grey"`,
	Phase_NoSocket:   `foreground="grey"`,
	Phase_Wat:        `foreground="yellow" background="red"`,
	Phase_NoPlan:     `foreground="grey"`,
	Phase_Queued:     `foreground="brown"`,
	Phase_InProgress: `foreground="brown"`,
	Phase_Rejected:   `foreground="red"`,
	Phase_Saving:     `foreground="brown"`,
	Phase_DoneGood:   `foreground="green"`,
	Phase_DoneNoGood: `foreground="red"`,
}
