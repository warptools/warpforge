package render

import (
	"bytes"
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
	gmnr := &nodeRenderer{
		mode:  m,
		width: physicalWidth,
	}
	gmr := renderer.NewRenderer(
		renderer.WithNodeRenderers(
			util.PrioritizedValue{Value: gmnr, Priority: 1},
		),
	)
	gmnr.gmr = gmr
	md := goldmark.New(
		// goldmark.WithExtensions(extension.GFM), // No GFM features are currently used in this part of our project.
		// goldmark.WithParserOptions(parser.WithAutoHeadingID()), // Not relevant to our ANSI output.
		goldmark.WithRenderer(gmr),
	)
	if err := md.Convert(markdown, wr); err != nil {
		panic(err)
	}
}

type Mode uint8

const (
	Mode_Markdown Mode = iota // Plain, honorable, and indentation-free markdown.
	Mode_ANSI                 // Text annotated with terminal ANSI codes for color, and indented fearlessly.  The terminal size is also detected and used for wrapping.
	Mode_ANSIdown             // Like Mode_ANSI, but we also re-include markdown annotations for block elements (inline element markers are dropped).  Indentation is still used.
	Mode_Plain                // Not implemented.  Spaced like Mode_ANSI, but without colors.
	Mode_HTML                 // Not implemented.  We actually use mode_markdown in our web docs pipeline.
)

// nodeRenderer is both the attachment point for all our funcs that render AST nodes,
// and also has the setup hook that goldmark uses to learn about them all.
//
// Confusingly, it is not, itself, a "goldmark renderer".
// You have to bounce through some goldmark constructors to get one of those.
// We also *store* one (that, confusingly, ends up looping control back to... us again)
// because we use it to render sub-sections of the document into buffers
// (which we need so that we can perform word wrap on those regions).
// Clear as mud?  Great, good.
type nodeRenderer struct {
	mode  Mode
	width int
	gmr   renderer.Renderer
}

// RegisterFuncs is to meet `goldmark/renderer.NodeRenderer`, and goldmark calls it to get further configuration done.
func (r *nodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {

	// blocks
	// reg.Register(ast.KindDocument, r.dumpDocument) // Uncomment for megadebugging.
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
	reg.Register(ast.KindEmphasis, r.renderEmphasis)
	// reg.Register(ast.KindImage, r.renderUnknown)
	// reg.Register(ast.KindLink, r.renderUnknown)
	reg.Register(ast.KindRawHTML, r.renderRawHTML) // not actually what this is :(
	reg.Register(ast.KindText, r.renderText)       // Most leaf nodes are ultimately this.  The simplest paragraph contains one element of text.
	// reg.Register(ast.KindString, r.renderUnknown) // I've yet to figure out what this is for.
}

func (r *nodeRenderer) dumpDocument(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		node.Dump(source, 2)
	}
	return ast.WalkContinue, nil
}

func (r *nodeRenderer) renderUnknown(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		w.WriteString(fmt.Sprintf("<%s ...>", node.Kind()))
	} else {
		w.WriteString(fmt.Sprintf("</%s>", node.Kind()))
	}
	if node.ChildCount() > 1 {
		w.WriteByte('\n')
	}
	return ast.WalkContinue, nil
}

func panicUnsupportedMode(m Mode) { panic(fmt.Errorf("unsupported mode %d", m)) }

// I'm attempting to use lipgloss below for style handling.
// However, I'm not totally enamoured.  It's doing a crazy amount of map lookups and memory allocations for every little thing.
// I might actually wanna switch to the termenv.Styled system instead.  Fewer layers.

func (r *nodeRenderer) renderHeading(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
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
		switch r.mode {
		case Mode_ANSI, Mode_ANSIdown:
			writeAnsi(w, ansiReset)
		}
		w.WriteByte('\n')
	}
	return ast.WalkContinue, nil
}

func (r *nodeRenderer) renderParagraph(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
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
			// First, do a new, separate rendering pass... into a buffer.
			// We'll skip children in the current walk after this, because we're handling them here.
			// Note that this is an independent *render* pass, but it's still using the same AST parse.  Nice API.
			// (The most degenerate placeholder for this would be `body := n.Text(source)`, but we want to actually render all the child elements!)
			var buf bytes.Buffer
			for child := n.FirstChild(); child != nil; child = child.NextSibling() {
				if err := r.gmr.Render(&buf, source, child); err != nil {
					return ast.WalkSkipChildren, err
				}
			}

			// Now that we've got all the child elements rendered into a buffer:
			// wordwrap it, indent that, and emit.
			body := buf.Bytes()
			left := 4 * nearestHeading
			// body = append(append([]byte("«"), body...), []byte("»")...) // Uncomment if debugging where paragraph edges are.
			if r.width > 0 {
				body = wordwrap.Bytes(body, r.width-2-left)
			}
			body = indent.Bytes(body, uint(left))
			w.Write(body)
			w.WriteByte('\n')

		default:
			panicUnsupportedMode(r.mode)
		}
	} else {
		w.WriteByte('\n')
	}
	return ast.WalkSkipChildren, nil
}

func (r *nodeRenderer) renderText(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		w.WriteString(string(node.Text(source)))
	}
	return ast.WalkContinue, nil
}

// In our domain, this isn't generally actually raw HTML.  It's just bracket characters.  We're gonna passthrough the plain text.
// ... And underline it, for fun.
func (r *nodeRenderer) renderRawHTML(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.RawHTML)
	if entering {
		writeAnsi(w, ansiUnderline)
		// Re-collect the raw text from the segments.
		// In practice I've only ever seen a single segment here, but this is what Dump does, so I presume this must be the right way to do it.
		for i := 0; i < n.Segments.Len(); i++ {
			segment := n.Segments.At(i)
			w.WriteString(string(segment.Value(source)))
		}
	} else {
		writeAnsi(w, 24)
	}
	return ast.WalkSkipChildren, nil
}

func (r *nodeRenderer) renderEmphasis(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	// FUTURE: use `node.(*ast.Emphasis).Level` to distinguish this further.
	if entering {
		md := "**"
		switch r.mode {
		case Mode_Markdown:
			w.WriteString(md)
		case Mode_ANSI, Mode_ANSIdown:
			writeAnsi(w, ansiBold)
		default:
			panicUnsupportedMode(r.mode)
		}
	} else {
		md := "**"
		switch r.mode {
		case Mode_Markdown:
			w.WriteString(md)
		case Mode_ANSI, Mode_ANSIdown:
			writeAnsi(w, 22)
		default:
			panicUnsupportedMode(r.mode)
		}
	}
	return ast.WalkContinue, nil
}
