package render

import "github.com/yuin/goldmark/ast"

func findHeading(node ast.Node /*srcForDebug []byte*/) int {
	for sib := node.PreviousSibling(); sib != nil; sib = sib.PreviousSibling() {
		switch sib.Kind() {
		case ast.KindHeading:
			return sib.(*ast.Heading).Level
		case ast.KindThematicBreak:
			return 0
		}
	}
	return 0
}
