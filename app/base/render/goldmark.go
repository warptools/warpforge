package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/muesli/reflow/indent"
	"github.com/muesli/reflow/wordwrap"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
	"golang.org/x/term"
)

// Render does what it says on the tin.
//
// The writer may be subject to feature detection to see if it's a terminal,
// and if so what it supports, if the mode parameter requests any ANSI behaviors.
//
// Render in plain markdown mode can be used as a sort of fmt'er.
// (This may be handy if your markdown source was produced by golang templates,
// which are notoriously unhelpful when it comes to letting you control whitespace.)
func Render(markdown []byte, wr io.Writer, m Mode) {
	physicalWidth := -1
	if fd, ok := wr.(interface{ Fd() uintptr }); ok {
		physicalWidth, _, _ = term.GetSize(int(fd.Fd()))
		if physicalWidth > 0 && physicalWidth < 60 {
			physicalWidth = 60
		}
	}
	// fmt.Printf(":: term width = %d\n", physicalWidth)
	md := goldmark.New(
		// goldmark.WithExtensions(extension.GFM), // No GFM features are currently used in this part of our project.
		// goldmark.WithParserOptions(parser.WithAutoHeadingID()), // Not relevant to our ANSI output.
		goldmark.WithRenderer(renderer.NewRenderer(
			renderer.WithNodeRenderers(
				util.PrioritizedValue{Value: &gmRenderer{m, physicalWidth}, Priority: 1},
			),
		)),
	)
	if err := md.Convert(markdown, wr); err != nil {
		panic(err)
	}
}

type Mode uint8

const (
	Mode_Markdown Mode = iota // Plain, honorable, and indentation-free markdown.
	Mode_ANSI                 // Text annotated with terminal ANSI codes for color, and indented fearlessly.  The terminal size is also detected and used for wrapping.
	Mode_ANSIdown             // Like Mode_ANSI, but we also re-include markdown annotations.  If leading whitespace and ANSI is stripped, you could parse this.
	Mode_Plain                // Not implemented.  Spaced like Mode_ANSI, but without colors.
	Mode_HTML                 // Not implemented.  We actually use mode_markdown in our web docs pipeline.
)

type gmRenderer struct {
	mode  Mode
	width int
}

// RegisterFuncs is to meet `goldmark/renderer.NodeRenderer`, and goldmark calls it to get further configuration done.
func (r *gmRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {

	// blocks
	reg.Register(ast.KindDocument, r.dumpDocument)
	reg.Register(ast.KindHeading, r.renderHeading)
	reg.Register(ast.KindParagraph, r.renderParagraph)
	// reg.Register(ast.KindParagraph, r.renderUnknown)
	// reg.Register(ast.KindBlockquote, r.renderUnknown)
	// reg.Register(ast.KindCodeBlock, r.renderUnknown)
	// reg.Register(ast.KindFencedCodeBlock, r.renderUnknown)
	// reg.Register(ast.KindHTMLBlock, r.renderUnknown)
	// reg.Register(ast.KindList, r.renderUnknown)
	// reg.Register(ast.KindListItem, r.renderUnknown)
	// reg.Register(ast.KindTextBlock, r.renderUnknown)
	// reg.Register(ast.KindThematicBreak, r.renderUnknown)

	// inlines
	// reg.Register(ast.KindAutoLink, r.renderUnknown)
	// reg.Register(ast.KindCodeSpan, r.renderUnknown)
	// reg.Register(ast.KindEmphasis, r.renderUnknown)
	// reg.Register(ast.KindImage, r.renderUnknown)
	// reg.Register(ast.KindLink, r.renderUnknown)
	reg.Register(ast.KindRawHTML, r.renderRawHTML) // not actually what this is :(
	reg.Register(ast.KindText, r.renderText)       // Most leaf nodes are ultimately this.  The simplest paragraph contains one element of text.
	// reg.Register(ast.KindString, r.renderUnknown) // I've yet to figure out what this is for.
}

func (r *gmRenderer) dumpDocument(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		node.Dump(source, 2)
	}
	return ast.WalkContinue, nil
}

func (r *gmRenderer) renderUnknown(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		w.WriteString(fmt.Sprintf("<%s ...>\n", node.Kind()))
	} else {
		w.WriteString(fmt.Sprintf("</%s>\n", node.Kind()))
	}
	return ast.WalkContinue, nil
}

func panicUnsupportedMode(m Mode) { panic(fmt.Errorf("unsupported mode %d", m)) }

// I'm attempting to use lipgloss below for style handling.
// However, I'm not totally enamoured.  It's doing a crazy amount of map lookups and memory allocations for every little thing.
// I might actually wanna switch to the termenv.Styled system instead.  Fewer layers.

func (r *gmRenderer) renderHeading(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Heading)
	if entering {
		mdPrefix := strings.Repeat("#", n.Level) + " "
		switch r.mode {
		case Mode_Markdown:
			w.WriteString(mdPrefix)
		case Mode_ANSI:
			mdPrefix = ""
			fallthrough
		case Mode_ANSIdown:
			switch n.Level {
			case 2:
				writeAnsi(w, ansiBold, ansiFgHiMagenta)
			case 3:
				w.WriteString(strings.Repeat(" ", 4))
				writeAnsi(w, ansiBold, ansiFgHiCyan)
			case 4:
				w.WriteString(strings.Repeat(" ", 8))
				writeAnsi(w, ansiBold, ansiFgHiBlue)
			}
			w.WriteString(mdPrefix)
		default:
			panicUnsupportedMode(r.mode)
		}
	} else {
		writeAnsi(w, ansiReset)
		w.WriteByte('\n')
	}
	return ast.WalkContinue, nil
}

func (r *gmRenderer) renderParagraph(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Paragraph)
	nearestHeading := findHeading(node)
	if entering {
		switch r.mode {
		case Mode_Markdown:
			w.WriteString(string(node.Text(source)))
			w.WriteByte('\n')
		case Mode_ANSI:
			fallthrough
		case Mode_ANSIdown:
			left := 4 * nearestHeading
			body := n.Text(source)
			// body = append(append([]byte("«"), body...), []byte("»")...) // Uncomment if debugging where paragraph edges are.
			if r.width > 0 {
				body = wordwrap.Bytes(body, r.width-2-left)
			}
			body = indent.Bytes(body, uint(left))
			w.Write(body)
			w.WriteByte('\n')
			// Still need to fix how this handles nested elements:
			//   - The .Text() getter produces fairly plain text encompassing all the children.  It's not the right thing.
			//     - We need to handle nested styling elements like emphasis.
			//       - ... which would be fine, except: That would require buffering all the children so I can wrap them.
			//         - It seems likely that'll require just implementing all the rendering for those without any more use of the goldmark ast walk system, because I can't change its buffering.
		default:
			panicUnsupportedMode(r.mode)
		}
	} else {
		w.WriteByte('\n')
	}
	return ast.WalkSkipChildren, nil
}

func (r *gmRenderer) renderText(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		w.WriteString(string(node.Text(source)))
	}
	return ast.WalkContinue, nil
}

// In our domain, this isn't generally actually raw HTML.  It's just bracket characters.  We're gonna passthrough the plain text.
func (r *gmRenderer) renderRawHTML(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		n := node.(*ast.RawHTML)
		// Re-collect the raw text from the segments.
		// In practice I've only ever seen a single segment here, but this is what Dump does, so I presume this must be the right way to do it.
		for i := 0; i < n.Segments.Len(); i++ {
			segment := n.Segments.At(i)
			w.WriteString(string(segment.Value(source)))
		}
	}
	return ast.WalkContinue, nil
}
