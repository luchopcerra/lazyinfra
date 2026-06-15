package views

import "github.com/charmbracelet/lipgloss"

var (
	sectionTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86"))
	tableHeader = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Underline(true)
	muted = lipgloss.NewStyle().
		Foreground(lipgloss.Color("244"))
	selected = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("238"))
	panel = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1, 2)
	badge = lipgloss.NewStyle().
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("60")).
		Padding(0, 1)
	errorLine = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))
	ok = lipgloss.NewStyle().
		Foreground(lipgloss.Color("42"))
	warn = lipgloss.NewStyle().
		Foreground(lipgloss.Color("214"))
)
