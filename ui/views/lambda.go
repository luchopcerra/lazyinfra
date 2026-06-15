package views

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type LambdaModel struct {
	functions []types.FunctionConfiguration
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

func (m *LambdaModel) SetFunctions(functions []types.FunctionConfiguration) {
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
	b.WriteString(tableHeader.Render("Function                       Runtime             Last Modified"))
	b.WriteString("\n")
	for i, fn := range m.functions {
		name := ""
		if fn.FunctionName != nil {
			name = *fn.FunctionName
		}
		runtime := string(fn.Runtime)
		lastMod := ""
		if fn.LastModified != nil {
			lastMod = *fn.LastModified
		}

		row := fmt.Sprintf("%-30s %-19s %s", name, runtime, lastMod)
		if i == m.selected {
			row = selected.Render(row)
		}
		b.WriteString(row + "\n")
	}
	b.WriteString("\n")
	b.WriteString(panel.Render("Invoke placeholder: press i in a future iteration to open a payload editor and invoke the selected function."))

	return lipgloss.NewStyle().Width(m.width).Render(b.String())
}
