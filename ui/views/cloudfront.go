package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	infraaws "lazyinfra/aws"
)

type CloudFrontModel struct {
	distributions []infraaws.Distribution
	pathInput     textinput.Model
	width         int
	height        int
	selected      int
}

func NewCloudFrontModel() CloudFrontModel {
	input := textinput.New()
	input.Placeholder = "/*"
	input.SetValue("/*")
	input.CharLimit = 256
	input.Width = 40

	return CloudFrontModel{pathInput: input}
}

func (m *CloudFrontModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.pathInput.Width = max(20, min(64, width-20))
}

func (m *CloudFrontModel) SetDistributions(distributions []infraaws.Distribution) {
	m.distributions = distributions
	if m.selected >= len(distributions) {
		m.selected = max(0, len(distributions)-1)
	}
}

func (m *CloudFrontModel) Update(msg tea.Msg) tea.Cmd {
	key, ok := msg.(tea.KeyMsg)
	if ok && len(m.distributions) > 0 {
		switch key.String() {
		case "up", "k":
			m.selected = max(0, m.selected-1)
		case "down", "j":
			m.selected = min(len(m.distributions)-1, m.selected+1)
		}
	}

	var cmd tea.Cmd
	m.pathInput, cmd = m.pathInput.Update(msg)
	return cmd
}

func (m CloudFrontModel) View() string {
	if len(m.distributions) == 0 {
		return muted.Render("Loading CloudFront distributions...")
	}

	var b strings.Builder
	b.WriteString(tableHeader.Render("Distribution   Status       Domain"))
	b.WriteString("\n")
	for i, dist := range m.distributions {
		status := dist.Status
		if status == "Deployed" {
			status = ok.Render(status)
		} else {
			status = warn.Render(status)
		}

		row := fmt.Sprintf("%-14s %-18s %s", dist.ID, status, dist.DomainName)
		if i == m.selected {
			row = selected.Render(row)
		}
		b.WriteString(row + "\n")
	}

	b.WriteString("\n")
	b.WriteString(panel.Render("Invalidation placeholder\n\nPath: " + m.pathInput.View() + "\n\nFuture action: submit an invalidation for the selected distribution."))

	return lipgloss.NewStyle().Width(m.width).Render(b.String())
}
