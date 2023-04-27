package helpgen

import (
	"strings"

	"github.com/urfave/cli/v2"
)

/*
	This file is almost an exact copy of functions from `urfave/cli`.
	Copying them is unavoidable due to their being unexported upstream.

	Avoid making changes in this file.
	There's no need to fear deviation from upstream,
	but there's probably no good reason to do so, either.
*/

func subtract(a, b int) int {
	return a - b
}

func indent(spaces int, v string) string {
	pad := strings.Repeat(" ", spaces)
	return pad + strings.Replace(v, "\n", "\n"+pad, -1)
}

func nindent(spaces int, v string) string {
	return "\n" + indent(spaces, v)
}

func wrap(input string, offset int, wrapAt int) string {
	var ss []string

	lines := strings.Split(input, "\n")

	padding := strings.Repeat(" ", offset)

	for i, line := range lines {
		if line == "" {
			ss = append(ss, line)
		} else {
			wrapped := wrapLine(line, offset, wrapAt, padding)
			if i == 0 {
				ss = append(ss, wrapped)
			} else {
				ss = append(ss, padding+wrapped)

			}

		}
	}

	return strings.Join(ss, "\n")
}

func wrapLine(input string, offset int, wrapAt int, padding string) string {
	if wrapAt <= offset || len(input) <= wrapAt-offset {
		return input
	}

	lineWidth := wrapAt - offset
	words := strings.Fields(input)
	if len(words) == 0 {
		return input
	}

	wrapped := words[0]
	spaceLeft := lineWidth - len(wrapped)
	for _, word := range words[1:] {
		if len(word)+1 > spaceLeft {
			wrapped += "\n" + padding + word
			spaceLeft = lineWidth - len(word)
		} else {
			wrapped += " " + word
			spaceLeft -= 1 + len(word)
		}
	}

	return wrapped
}

func offset(input string, fixed int) int {
	return len(input) + fixed
}

// this function tries to find the max width of the names column
// so say we have the following rows for help
//
//	foo1, foo2, foo3  some string here
//	bar1, b2 some other string here
//
// We want to offset the 2nd row usage by some amount so that everything
// is aligned
//
//	foo1, foo2, foo3  some string here
//	bar1, b2          some other string here
//
// to find that offset we find the length of all the rows and use the max
// to calculate the offset
func offsetCommands(cmds []*cli.Command, fixed int) int {
	var max int = 0
	for _, cmd := range cmds {
		s := strings.Join(cmd.Names(), ", ")
		if len(s) > max {
			max = len(s)
		}
	}
	return max + fixed
}
