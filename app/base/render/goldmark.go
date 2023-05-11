package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
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
	tr := lipgloss.NewRenderer(wr)
	if tr.Output().Profile == termenv.Ascii { // no it isn't, it's just somebody piping to `head` or whatever.
		tr.Output().Profile = termenv.ANSI
	}
	physicalWidth := -1
	if fd, ok := wr.(interface{ Fd() uintptr }); ok {
		physicalWidth, _, _ = term.GetSize(int(fd.Fd()))
	}
	// fmt.Printf(":: term width = %d\n", physicalWidth)
	style := style{
		SectionHeading: tr.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("123")).
			Background(lipgloss.Color("#FF44FF")),
		EntryHeadingCommand: tr.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("128")),
		EntryHeadingFlag: tr.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("140")),
		Paragraph: tr.NewStyle().
			Width(physicalWidth). // Must be provided to get wordwrap.
			Margin(1, 0),
	}
	md := goldmark.New(
		// goldmark.WithExtensions(extension.GFM), // No GFM features are currently used in this part of our project.
		// goldmark.WithParserOptions(parser.WithAutoHeadingID()), // Not relevant to our ANSI output.
		goldmark.WithRenderer(renderer.NewRenderer(
			renderer.WithNodeRenderers(
				util.PrioritizedValue{Value: &gmRenderer{m, tr, style}, Priority: 1},
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
	tr    *lipgloss.Renderer
	style style
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
	reg.Register(ast.KindRawHTML, r.renderUnknown) // not actually what this is :(
	// reg.Register(ast.KindText, r.renderUnknown)
	// reg.Register(ast.KindString, r.renderUnknown)
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
				w.WriteString(r.style.SectionHeading.Render(mdPrefix))
			case 3:
				w.WriteString(r.style.EntryHeadingCommand.Render(mdPrefix))
			case 4:
				w.WriteString(r.style.EntryHeadingFlag.Render(mdPrefix))
			}
		default:
			panicUnsupportedMode(r.mode)
		}
	} else {
		w.WriteByte('\n')
		// TODO I don't know how to "clear" with lipgloss, nor if I need to.
		//   ... yeah, termenv puts CSI+ResetSeq on the end of every single thing.  That's... actually not at all helpful.
	}
	return ast.WalkContinue, nil
}

func (r *gmRenderer) renderParagraph(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Paragraph)
	nearestHeading := findHeading(node)
	if entering {
		switch r.mode {
		case Mode_Markdown:
			w.WriteByte('\n')
			w.WriteString(string(node.Text(source)))
		case Mode_ANSI:
			fallthrough
		case Mode_ANSIdown:
			w.WriteString(r.style.Paragraph.MarginLeft(4 * nearestHeading).Render("«" + string(n.Text(source)) + "»"))
			// Multiple problems above:
			//   - Margins in lipgloss don't actually do what I want :(  They wrap before adding margins.
			//   - Margins in lipgloss also cause it to padd things out to an even size on the right, which... doesn't really make things better, in this scenario.
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
	return ast.WalkContinue, nil
}
