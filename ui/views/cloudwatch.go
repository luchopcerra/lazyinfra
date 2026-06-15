package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	infraaws "lazyinfra/aws"
)

type CloudWatchModel struct {
	groups   []infraaws.LogGroup
	lines    []string
	tail     viewport.Model
	width    int
	height   int
	selected int
}

func NewCloudWatchModel() CloudWatchModel {
	return CloudWatchModel{tail: viewport.New(80, 12)}
}

func (m *CloudWatchModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.tail.Width = max(30, width-2)
	m.tail.Height = max(8, height-12)
}

func (m *CloudWatchModel) SetLogGroups(groups []infraaws.LogGroup) {
	m.groups = groups
	if m.selected >= len(groups) {
		m.selected = max(0, len(groups)-1)
	}
}

func (m *CloudWatchModel) AppendLines(lines []string) {
	m.lines = append(m.lines, lines...)
	m.tail.SetContent(m.renderLogLines())
	m.tail.GotoBottom()
}

func (m *CloudWatchModel) Update(msg tea.Msg) tea.Cmd {
	key, ok := msg.(tea.KeyMsg)
	if !ok || len(m.groups) == 0 {
		var cmd tea.Cmd
		m.tail, cmd = m.tail.Update(msg)
		return cmd
	}

	switch key.String() {
	case "up", "k":
		m.selected = max(0, m.selected-1)
	case "down", "j":
		m.selected = min(len(m.groups)-1, m.selected+1)
	}

	var cmd tea.Cmd
	m.tail, cmd = m.tail.Update(msg)
	return cmd
}

func (m CloudWatchModel) View() string {
	var b strings.Builder
	b.WriteString(sectionTitle.Render("Log Groups"))
	b.WriteString("\n")

	if len(m.groups) == 0 {
		b.WriteString(muted.Render("Loading log groups...") + "\n")
	} else {
		for i, group := range m.groups {
			row := fmt.Sprintf("%-40s %8d bytes  retention=%dd", group.Name, group.StoredBytes, group.RetentionDays)
			if i == m.selected {
				row = selected.Render(row)
			}
			b.WriteString(row + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(sectionTitle.Render("Tail"))
	b.WriteString("\n")
	if len(m.lines) == 0 {
		m.tail.SetContent(muted.Render("No active stream. Press t to append sample log events."))
	}
	b.WriteString(panel.Width(m.width - 2).Render(m.tail.View()))

	return lipgloss.NewStyle().Width(m.width).Render(b.String())
}

func (m CloudWatchModel) renderLogLines() string {
	out := make([]string, 0, len(m.lines))
	for _, line := range m.lines {
		if strings.Contains(line, "ERROR") || strings.Contains(line, "Exception") {
			out = append(out, errorLine.Render(line))
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
