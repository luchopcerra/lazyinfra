package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	infraaws "lazyinfra/aws"
)

type LambdaModel struct {
	functions []infraaws.LambdaFunction
	selected  int
	width     int
	height    int
}

func NewLambdaModel() LambdaModel {
	return LambdaModel{}
}

func (m *LambdaModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *LambdaModel) SetFunctions(functions []infraaws.LambdaFunction) {
	m.functions = functions
	if m.selected >= len(functions) {
		m.selected = max(0, len(functions)-1)
	}
}

func (m *LambdaModel) Update(msg tea.Msg) tea.Cmd {
	key, ok := msg.(tea.KeyMsg)
	if !ok || len(m.functions) == 0 {
		return nil
	}

	switch key.String() {
	case "up", "k":
		m.selected = max(0, m.selected-1)
	case "down", "j":
		m.selected = min(len(m.functions)-1, m.selected+1)
	}
	return nil
}

func (m LambdaModel) View() string {
	if len(m.functions) == 0 {
		return muted.Render("Loading Lambda functions...")
	}

	var b strings.Builder
	b.WriteString(tableHeader.Render("Function                       Runtime       Memory  Last Modified"))
	b.WriteString("\n")
	for i, fn := range m.functions {
		row := fmt.Sprintf("%-30s %-12s %4dMB  %s", fn.Name, fn.Runtime, fn.MemoryMB, fn.LastModified)
		if i == m.selected {
			row = selected.Render(row)
		}
		b.WriteString(row + "\n")
	}
	b.WriteString("\n")
	b.WriteString(panel.Render("Invoke placeholder: press i in a future iteration to open a payload editor and invoke the selected function."))

	return lipgloss.NewStyle().Width(m.width).Render(b.String())
}
