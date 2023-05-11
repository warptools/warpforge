package render

import (
	"io"
	"strconv"
)

// Yet another quick-n-dirty ansi color code table.
//
// Why not use ${X}?
//
// - https://github.com/mgutz/ansi -- I dunno, it's not really simplifying anything.
// - https://github.com/charmbracelet/lipgloss -- It's both way too much and not enough.  Much abstraction, little composability.
// - https://github.com/fatih/color -- I'm bothered that a simple utility library finds it appropriate to read environment variables.
// - https://github.com/shiena/ansicolor/ -- It's... rewriting ANSI to windows codes on the fly?  That's solving problems I don't have.
// - https://github.com/k0kubun/go-ansi -- Same as above.
// - https://github.com/muesli/termenv -- Close to the mark, but it's spending a lot of time wrapping the output, and the style code itself is dwarfed by this.  And then it starts reading env vars too.  You can give it a mock environment to make it stop, but jeesh.
// - https://github.com/muesli/ansi -- It's a deduplicator?  That's solving problems I don't have.
// - https://github.com/jedib0t/go-pretty/ -- Has its own tiny thing for this too, actually.  Nice.
//     - ... the 'text' package is _so close_, but then it's got dependencies on JSON too, for... reasons?  What?  And a random dep on github.com/go-openapi/strfmt ?  Why?  Why.
// - Are there others?  It's taking me longer to inspect these than it's going to take to write one.
//
// Really, we just need some dingdang strings, and a way to compose them.
// Ideally to a stream, and just with printf, because there's no need to be slow.
//
// People make this WAY too complicated.
// Those are pretty simple goals to achieve.

type ansiColor int

var (
	ansi_CSI_str   = "\x1b["
	ansi_CSI_bytes = []byte(ansi_CSI_str)
	ansi_SGR       = 'm'
	ansi_SGR_bytes = []byte{byte(ansi_SGR)}
)

func writeAnsi(wr io.Writer, codes ...ansiColor) (n int, err error) {
	var n2 int
	n2, err = wr.Write(ansi_CSI_bytes)
	n += n2
	if err != nil {
		return
	}
	for i, code := range codes {
		if i > 0 {
			n2, err = wr.Write([]byte{';'})
			n += n2
			if err != nil {
				return
			}
		}
		n2, err = wr.Write([]byte(strconv.Itoa(int(code)))) // TODO some cycles would be saved if we byteify these in advance.
		n += n2
		if err != nil {
			return
		}
	}
	n2, err = wr.Write(ansi_SGR_bytes)
	n += n2
	return
}

// Attributes
// (We'll call these "colors" still because they fall in the "m" feature of ANSI codes.)
const (
	ansiReset ansiColor = iota
	ansiBold
	ansiFaint
	ansiItalic
	ansiUnderline
	ansiBlinkSlow
	ansiBlinkRapid
	ansiReverseVideo
	ansiConcealed
	ansiCrossedOut
)

// Foreground colors
const (
	ansiFgBlack ansiColor = iota + 30
	ansiFgRed
	ansiFgGreen
	ansiFgYellow
	ansiFgBlue
	ansiFgMagenta
	ansiFgCyan
	ansiFgWhite
)

// Foreground Hi-Intensity colors
const (
	ansiFgHiBlack ansiColor = iota + 90
	ansiFgHiRed
	ansiFgHiGreen
	ansiFgHiYellow
	ansiFgHiBlue
	ansiFgHiMagenta
	ansiFgHiCyan
	ansiFgHiWhite
)

// Background colors
const (
	ansiBgBlack ansiColor = iota + 40
	ansiBgRed
	ansiBgGreen
	ansiBgYellow
	ansiBgBlue
	ansiBgMagenta
	ansiBgCyan
	ansiBgWhite
)

// Background Hi-Intensity colors
const (
	ansiBgHiBlack ansiColor = iota + 100
	ansiBgHiRed
	ansiBgHiGreen
	ansiBgHiYellow
	ansiBgHiBlue
	ansiBgHiMagenta
	ansiBgHiCyan
	ansiBgHiWhite
)
