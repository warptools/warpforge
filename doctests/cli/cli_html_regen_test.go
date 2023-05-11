package doctests_cli

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/charmbracelet/glamour"

	wfapp "github.com/warptools/warpforge/app"
)

func TestRegenerate(t *testing.T) {

	//wfapp.App.Description = "A longer, multi-line and multi-paragraph description may go here."
	//"> This is a blockquote\n> with multiple lines\n>> >> > > now indented\n> back again\n> ```\n> codeblocks??\n> ```\n"

	wfapp.App.Writer = os.Stdout
	wfapp.App.ErrWriter = os.Stderr
	_ = wfapp.App.Run([]string{"-h"})

	fmt.Println("--------")

	// style := glamour.ASCIIStyleConfig // a lot of things seem ignored when using glamour.DarkStyleConfig ?  namely codeblock prefix and suffix
	style := glamour.DarkStyleConfig
	stringPtr := func(s string) *string { return &s }
	uintPtr := func(u uint) *uint { return &u }
	style.Document.Margin = uintPtr(0)
	style.Paragraph.Margin = uintPtr(6)
	style.Code.Prefix = "`"
	style.Code.Suffix = "`"
	style.CodeBlock.Margin = uintPtr(8)
	style.CodeBlock.Prefix = "```\n"
	style.CodeBlock.Suffix = "```\n"
	//style.CodeBlock.Chroma = nil // Presence of chroma oversides codeblock prefix and suffix...?  Seems like something I'd consider a bug?  Report upstream?
	style.H3.BlockSuffix = " "
	style.H3.Margin = uintPtr(2)
	style.H3.Color = stringPtr("135")
	style.H4.BlockSuffix = " "
	style.H4.Margin = uintPtr(2)
	style.H4.Color = stringPtr("67")
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

}
