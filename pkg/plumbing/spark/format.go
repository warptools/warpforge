package spark

import (
	"github.com/warptools/warpforge/pkg/workspaceapi"
)

type Style string

const (
	MarkupSimple  Style = "simple"
	MarkupBash    Style = "bash"
	MarkupPango   Style = "pango"
	MarkupAnsi    Style = "ansi"
	DefaultMarkup       = MarkupSimple
)

// Phase is a 3-character code for all output conditions.
type Phase string

const (
	Phase_NoModule   Phase = "nop" // spark-side only: cwd doesn't look like a campsite.
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

	default:
		return Phase_Wat
	}
}

const AnsiColorReset = "\x1B[0m"

var dasAnsiColorMap = map[Phase]string{
	Phase_NoModule:   "\x1B[1;30m",                // grey
	Phase_NoSocket:   "\x1B[1;30m",                // grey
	Phase_Wat:        "\x1B[5m\x1B[41m\x1B[1;33m", // blink yellow, red bg
	Phase_NoPlan:     "\x1B[1;30m",                // grey
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
	Phase_DoneNoGood: "↯↯↯",
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
