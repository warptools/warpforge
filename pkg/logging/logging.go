package logging

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
)

type Logger struct {
	out     io.Writer
	err     io.Writer
	verbose bool
}

func DefaultLogger() Logger {
	return Logger{
		out:     os.Stdout,
		err:     os.Stderr,
		verbose: false,
	}
}

func NewLogger(out, err io.Writer, verbose bool) Logger {
	return Logger{
		out,
		err,
		verbose,
	}
}

func (l *Logger) Out(f string, args ...interface{}) {
	fmt.Fprintf(l.out, f+"\n", args...)
}

func (l *Logger) OutRaw(s string) {
	fmt.Fprintf(l.out, "%s", s)
}

func (l *Logger) Info(tag string, f string, args ...interface{}) {
	print(l.err, color.New(color.FgHiGreen), tag, f, args...)
}

func (l *Logger) Debug(tag string, f string, args ...interface{}) {
	if l.verbose {
		print(l.err, color.New(color.FgGreen), tag, f, args...)
	}
}

func print(w io.Writer, tagColor *color.Color, tag, f string, args ...interface{}) {
	str := fmt.Sprintf(f, args...)
	for _, line := range strings.Split(str, "\n") {
		fmt.Fprintf(w, "%s  %s\n",
			tagColor.Sprint(tag),
			color.WhiteString(line))
	}
}

type Writer struct {
	pipe io.Writer
	tag  string
}

func (l *Logger) InfoWriter(tag string) *Writer {
	return &Writer{
		pipe: l.err,
		tag:  tag,
	}
}

func (w *Writer) Write(data []byte) (n int, err error) {
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fmt.Fprintf(w.pipe, "%s  %s\n",
			color.HiYellowString(w.tag),
			color.HiWhiteString(line))
	}
	return len(data), nil
}
