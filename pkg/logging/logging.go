package logging

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/fatih/color"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec"
	"github.com/ipld/go-ipld-prime/codec/dagjson"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/warpfork/warpforge/wfapi"

	rfmtjson "github.com/polydawn/refmt/json"
)

type Logger struct {
	out     io.Writer
	err     io.Writer
	verbose bool
	json    bool
	quiet   bool
}

func jsonEncoder(n datamodel.Node, w io.Writer) error {
	return dagjson.Marshal(n, rfmtjson.NewEncoder(w, rfmtjson.EncodeOptions{
		Line:   []byte{' '},
		Indent: []byte{},
	}), dagjson.EncodeOptions{
		EncodeLinks: false,
		EncodeBytes: false,
		MapSortMode: codec.MapSortMode_None,
	})
}

func DefaultLogger() Logger {
	return Logger{
		out:     os.Stdout,
		err:     os.Stderr,
		verbose: false,
	}
}

func NewLogger(out, err io.Writer, json bool, quiet bool, verbose bool) Logger {
	return Logger{
		out,
		err,
		verbose,
		json,
		quiet,
	}
}

func (l *Logger) Out(f string, args ...interface{}) {
	fmt.Fprintf(l.out, f+"\n", args...)
}

func (l *Logger) OutRaw(s string) {
	fmt.Fprintf(l.out, "%s", s)
}

func (l *Logger) Info(tag string, f string, args ...interface{}) {
	if l.quiet {
		return
	}
	if l.json {
		apiLog(l.out, f, args...)
	} else {
		print(l.err, color.New(color.FgHiGreen), tag, f, args...)
	}
}

func (l *Logger) Debug(tag string, f string, args ...interface{}) {
	if l.quiet {
		return
	}
	if l.verbose {
		if l.json {
			apiLog(l.out, f, args...)
		} else {
			print(l.err, color.New(color.FgGreen), tag, f, args...)
		}
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

func stripAnsiAndWhitespace(s string) string {
	// remove ansi color characters
	ansiRe := regexp.MustCompile("\x1b[[0-9;]*[mGKH]")
	s = ansiRe.ReplaceAllString(s, "")

	// replace newlines and tab sequences with a single space
	whitespaceRe := regexp.MustCompile("(\n|\t)+")
	s = whitespaceRe.ReplaceAllString(s, " ")

	s = strings.TrimSpace(s)

	return s
}

func apiLog(w io.Writer, f string, args ...interface{}) {
	if f == "" {
		// empty strings are useful for pretty formatting, but useless for API output
		// ignore
		return
	}
	log := wfapi.LogOutput{
		Msg: wfapi.LogString(stripAnsiAndWhitespace(fmt.Sprintf(f, args...))),
	}
	out := wfapi.ApiOutput{
		Log: &log,
	}

	apiWrite(w, out)
}

func apiOutput(w io.Writer, s string) {
	str := wfapi.OutputString(s)
	out := wfapi.ApiOutput{
		Output: &str,
	}

	apiWrite(w, out)
}

func apiWrite(w io.Writer, out wfapi.ApiOutput) {
	serial, err := ipld.Marshal(jsonEncoder, &out, wfapi.TypeSystem.TypeByName("ApiOutput"))
	if err != nil {
		panic("error serializing json log?!")
	}

	fmt.Fprintf(w, "%s\n", string(serial))
}

func (l *Logger) PrintRunRecord(tag string, rr wfapi.RunRecord, memoized bool) {
	if l.json {
		out := wfapi.ApiOutput{
			RunRecord: &rr,
		}
		apiWrite(l.out, out)
	} else {
		headline := "RunRecord"
		if memoized {
			headline = "RunRecord (memoized)"
			l.Info(tag, "skipping execution, formula memoized")
		}
		l.Info(tag, "%s:\n\t%s = %s\n\t%s = %s\n\t%s = %s\n\t%s = %s\n\t%s:",
			headline,
			color.HiBlueString("GUID"),
			color.WhiteString(rr.Guid),
			color.HiBlueString("FormulaID"),
			color.WhiteString(rr.FormulaID),
			color.HiBlueString("Exitcode"),
			color.WhiteString(fmt.Sprintf("%d", rr.Exitcode)),
			color.HiBlueString("Time"),
			color.WhiteString(fmt.Sprintf("%d", rr.Time)),
			color.HiBlueString("Results"),
		)

		for k, v := range rr.Results.Values {
			l.Info(tag, "\t\t%s: %s", k, v.WareID)
		}
	}
}

func (l *Logger) PrintPlotResults(tag string, pr wfapi.PlotResults) {
	if l.json {
		out := wfapi.ApiOutput{
			PlotResults: &pr,
		}
		apiWrite(l.out, out)
	} else {
		l.Info(tag, "outputs:")
		for name, wareId := range pr.Values {
			l.Info(tag, "\t%s: %s",
				color.HiBlueString(string(name)),
				color.WhiteString(wareId.String()))
		}
	}
}

type Writer struct {
	pipe io.Writer
	tag  string
	raw  bool
	json bool
}

func (l *Logger) InfoWriter(tag string) *Writer {
	return &Writer{
		pipe: l.err,
		tag:  tag,
		raw:  false,
		json: l.json,
	}
}

func (l *Logger) RawWriter() *Writer {
	return &Writer{
		pipe: l.out,
		tag:  "",
		raw:  true,
		json: l.json,
	}
}

// Write raw bytes to to the log
//
// Errors: none -- return value used keep io.Writer interface
func (w *Writer) Write(data []byte) (n int, err error) {
	if w.json {
		apiOutput(w.pipe, string(data))
	} else {
		if w.raw {
			fmt.Fprintf(w.pipe, "%s", data)
		} else {
			for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
				fmt.Fprintf(w.pipe, "%s  %s\n",
					color.HiGreenString(w.tag),
					line)
			}
		}
	}
	return len(data), nil
}
