package render

import "github.com/charmbracelet/lipgloss"

type style struct {
	SectionHeading      lipgloss.Style
	EntryHeadingCommand lipgloss.Style
	EntryHeadingFlag    lipgloss.Style

	Paragraph lipgloss.Style
}
