package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	infraaws "lazyinfra/aws"
)

type APIModel struct {
	apis   []infraaws.API
	width  int
	height int
}

func NewAPIModel() APIModel {
	return APIModel{}
}

func (m *APIModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *APIModel) SetAPIs(apis []infraaws.API) {
	m.apis = apis
}

func (m *APIModel) Update(tea.Msg) tea.Cmd {
	return nil
}

func (m APIModel) View() string {
	if len(m.apis) == 0 {
		return muted.Render("Loading APIs...")
	}

	var b strings.Builder
	for _, api := range m.apis {
		b.WriteString(sectionTitle.Render(fmt.Sprintf("%s  %s", api.Name, badge.Render(api.Protocol))))
		b.WriteString("\n")
		for _, route := range api.Routes {
			b.WriteString(fmt.Sprintf("  %s %-24s -> %s\n", methodStyle(route.Method), route.Path, route.LambdaFunction))
		}
		b.WriteString("\n")
	}

	return lipgloss.NewStyle().Width(m.width).Render(strings.TrimRight(b.String(), "\n"))
}

func methodStyle(method string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("31")).
		Padding(0, 1).
		Width(6).
		Align(lipgloss.Center).
		Render(method)
}
