package doctests_cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/charmbracelet/glamour"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"

	wfapp "github.com/warptools/warpforge/app"
)

func TestRegenerate(t *testing.T) {
	wfapp.App.Description = "A longer, multi-line and multi-paragraph description may go here."
	//"> This is a blockquote\n> with multiple lines\n>> >> > > now indented\n> back again\n> ```\n> codeblocks??\n> ```\n"

	wfapp.App.Writer = os.Stdout
	wfapp.App.ErrWriter = os.Stderr
	_ = wfapp.App.Run([]string{"-h"})

	fmt.Println("--------")

	// style := glamour.ASCIIStyleConfig // a lot of things seem ignored when using glamour.DarkStyleConfig ?  namely codeblock prefix and suffix
	style := glamour.DarkStyleConfig
	stringPtr := func(s string) *string { return &s }
	uintPtr := func(u uint) *uint { return &u }
	style.Document.Margin = uintPtr(2)
	style.Paragraph.Margin = uintPtr(2)
	style.Code.Prefix = "`"
	style.Code.Suffix = "`"
	style.CodeBlock.Margin = uintPtr(6)
	style.CodeBlock.Prefix = "```\n"
	style.CodeBlock.Suffix = "```\n"
	//style.CodeBlock.Chroma = nil // Presence of chroma oversides codeblock prefix and suffix...?  Seems like something I'd consider a bug?  Report upstream?
	style.H4.BlockSuffix = " "
	style.H4.Margin = uintPtr(4)
	style.H4.Color = stringPtr("139")
	style.Table.CenterSeparator = stringPtr("x")

	//style = glamour.ASCIIStyleConfig

	r, _ := glamour.NewTermRenderer(
		//glamour.WithAutoStyle(),
		glamour.WithStyles(style),
		glamour.WithWordWrap(80), // this does things that are wrong?  setting it to a large number produces insanity and many excessive lines.  // ... okay maybe only for code blocks?  // ... nope not even just code blocks.  // OKAY NO, it's something weird to vscode's terminal.  investigation aborting, not critical.
	)

	wfapp.App.Writer = r
	wfapp.App.ErrWriter = r
	_ = wfapp.App.Run([]string{"-h"})

	r.Close()
	io.Copy(os.Stdout, r)

	fmt.Println("--------")

	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRenderer(goldmarkRendererToANSI()),
	)
	var buf bytes.Buffer
	wfapp.App.Writer = &buf
	wfapp.App.ErrWriter = &buf
	_ = wfapp.App.Run([]string{"-h"})
	if err := md.Convert(buf.Bytes(), os.Stdout); err != nil {
		panic(err)
	}

}

func goldmarkRendererToANSI() renderer.Renderer {
	return renderer.NewRenderer(
		renderer.WithNodeRenderers(util.PrioritizedValue{Value: &gmRenderer{}, Priority: 1}),
	)
}

type gmRenderer struct {
}

// RegisterFuncs is to meet `goldmark/renderer.NodeRenderer`, and goldmark calls it to get further configuration done.
func (r *gmRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindDocument, r.renderDocument)
	reg.Register(ast.KindHeading, r.renderHeading)
	reg.Register(ast.KindParagraph, r.renderParagraph)
}

func (r *gmRenderer) renderDocument(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	//node.Dump(source, 0)
	return ast.WalkContinue, nil
}

func (r *gmRenderer) renderHeading(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Heading)
	if entering {
		_, _ = w.WriteString("<h")
		_ = w.WriteByte("0123456"[n.Level])
		_ = w.WriteByte('>')
	} else {
		_, _ = w.WriteString("</h")
		_ = w.WriteByte("0123456"[n.Level])
		_, _ = w.WriteString(">\n")
	}
	return ast.WalkContinue, nil
}

func (r *gmRenderer) renderParagraph(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {

	if entering {
		_, _ = w.WriteString("<p nearestHeading=")
		_ = w.WriteByte("0123456"[findHeading(node)])
		_, _ = w.WriteString(">")
	} else {
		_, _ = w.WriteString("</p>\n")
	}
	return ast.WalkContinue, nil
}

func findHeading(node ast.Node) int {
	for sib := node.PreviousSibling(); sib != nil; sib = node.PreviousSibling() {
		switch sib.Kind() {
		case ast.KindHeading:
			return sib.(*ast.Heading).Level
		case ast.KindThematicBreak:
			return 0
		}
	}
	return 0
}

// One could also imagine a findHeadingTree function, which returns pointers to the nearest h3, h2, etc, thus giving you access to any attributes on each.
// That could be the basis for stylesheets that I wouldn't quite call cascading, but would be sufficiently powerful for pretty much everything I can currently imagine wanting to do.
