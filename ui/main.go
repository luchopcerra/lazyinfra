package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	infraaws "lazyinfra/aws"
	"lazyinfra/ui/views"

	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

type service int

const (
	serviceAPIGateway service = iota
	serviceLambda
	serviceCloudWatch
	serviceCloudFront
)

var services = []struct {
	id    service
	title string
	desc  string
}{
	{serviceAPIGateway, "API Gateway", "routes and integrations"},
	{serviceLambda, "AWS Lambda", "functions and invoke"},
	{serviceCloudWatch, "CloudWatch", "logs and tailing"},
	{serviceCloudFront, "CloudFront", "distributions and invalidations"},
}

type Model struct {
	client *infraaws.AWSClient

	sidebar list.Model
	active  service
	width   int
	height  int

	api        views.APIModel
	lambda     views.LambdaModel
	cloudwatch views.CloudWatchModel
	cloudfront views.CloudFrontModel
	lastErr    error
}

type menuItem struct {
	title string
	desc  string
	id    service
}

func (m menuItem) Title() string       { return m.title }
func (m menuItem) Description() string { return m.desc }
func (m menuItem) FilterValue() string { return m.title }

func NewModel(client *infraaws.AWSClient) Model {
	items := make([]list.Item, 0, len(services))
	for _, svc := range services {
		items = append(items, menuItem{title: svc.title, desc: svc.desc, id: svc.id})
	}

	sidebar := list.New(items, list.NewDefaultDelegate(), 28, 20)
	sidebar.Title = "lazyinfra"
	sidebar.SetShowStatusBar(false)
	sidebar.SetFilteringEnabled(false)
	sidebar.SetShowHelp(false)

	return Model{
		client:     client,
		sidebar:    sidebar,
		active:     serviceAPIGateway,
		api:        views.NewAPIModel(),
		lambda:     views.NewLambdaModel(),
		cloudwatch: views.NewCloudWatchModel(),
		cloudfront: views.NewCloudFrontModel(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadAPIs(),
		m.loadLogGroups(),
		m.loadDistributions(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		sidebarWidth, contentWidth := m.panelWidths()
		m.sidebar.SetSize(sidebarWidth-2, msg.Height-2)
		m.api.SetSize(contentWidth, msg.Height-2)
		m.lambda.SetSize(contentWidth, msg.Height-2)
		m.cloudwatch.SetSize(contentWidth, msg.Height-2)
		m.cloudfront.SetSize(contentWidth, msg.Height-2)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "1":
			m.sidebar.Select(0)
			m.active = serviceAPIGateway
		case "2":
			m.sidebar.Select(1)
			m.active = serviceLambda
			cmds = append(cmds, m.loadLambdaFunctions())
		case "3":
			m.sidebar.Select(2)
			m.active = serviceCloudWatch
		case "4":
			m.sidebar.Select(3)
			m.active = serviceCloudFront
		case "enter":
			if item, ok := m.sidebar.SelectedItem().(menuItem); ok {
				m.active = item.id
				if m.active == serviceLambda {
					cmds = append(cmds, m.loadLambdaFunctions())
				}
			}
		case "r":
			cmds = append(cmds, m.refreshActive())
		case "t":
			if m.active == serviceCloudWatch {
				cmds = append(cmds, m.tailLogs())
			}
		}
	case errMsg:
		m.lastErr = fmt.Errorf("%s: %w", msg.Service, msg.Err)
	case lambdaListLoadedMsg:
		m.lastErr = nil
		m.lambda.SetFunctions([]types.FunctionConfiguration(msg))
	case apiListLoadedMsg:
		m.lastErr = nil
		m.api.SetAPIs([]infraaws.API(msg))
	case logGroupsLoadedMsg:
		m.lastErr = nil
		m.cloudwatch.SetLogGroups([]infraaws.LogGroup(msg))
	case logLinesAppendedMsg:
		m.lastErr = nil
		m.cloudwatch.AppendLines([]string(msg))
	case distributionsLoadedMsg:
		m.lastErr = nil
		m.cloudfront.SetDistributions([]infraaws.Distribution(msg))
	}

	var cmd tea.Cmd
	m.sidebar, cmd = m.sidebar.Update(msg)
	cmds = append(cmds, cmd)

	switch m.active {
	case serviceAPIGateway:
		cmds = append(cmds, m.api.Update(msg))
	case serviceLambda:
		cmds = append(cmds, m.lambda.Update(msg))
	case serviceCloudWatch:
		cmds = append(cmds, m.cloudwatch.Update(msg))
	case serviceCloudFront:
		cmds = append(cmds, m.cloudfront.Update(msg))
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading lazyinfra..."
	}

	sidebar := styles.sidebar.Render(m.sidebar.View())
	content := styles.content.Width(m.contentWidth()).Render(m.activeView())

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)
}

func (m Model) activeView() string {
	header := styles.header.Render(m.activeTitle())
	body := ""

	switch m.active {
	case serviceAPIGateway:
		body = m.api.View()
	case serviceLambda:
		body = m.lambda.View()
	case serviceCloudWatch:
		body = m.cloudwatch.View()
	case serviceCloudFront:
		body = m.cloudfront.View()
	}

	footerParts := []string{"1-4 switch", "enter select", "r refresh", "q quit"}
	if m.active == serviceCloudWatch {
		footerParts = append(footerParts, "t append sample logs")
	}
	if m.lastErr != nil {
		footerParts = append(footerParts, styles.error.Render(m.lastErr.Error()))
	}

	footer := styles.footer.Render(strings.Join(footerParts, "  |  "))
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m Model) activeTitle() string {
	for _, svc := range services {
		if svc.id == m.active {
			return svc.title
		}
	}
	return "lazyinfra"
}

func (m Model) panelWidths() (int, int) {
	sidebarWidth := max(24, m.width/5)
	contentWidth := max(40, m.width-sidebarWidth-4)
	return sidebarWidth, contentWidth
}

func (m Model) contentWidth() int {
	_, width := m.panelWidths()
	return width
}

func (m Model) refreshActive() tea.Cmd {
	switch m.active {
	case serviceAPIGateway:
		return m.loadAPIs()
	case serviceLambda:
		return m.loadLambdaFunctions()
	case serviceCloudWatch:
		return m.loadLogGroups()
	case serviceCloudFront:
		return m.loadDistributions()
	default:
		return nil
	}
}

func (m Model) loadLambdaFunctions() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		functions, err := m.client.FetchLambdas(ctx)
		if err != nil {
			return errMsg{Service: "lambda", Err: err}
		}
		return lambdaListLoadedMsg(functions)
	}
}

func (m Model) loadAPIs() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		apis, err := m.client.ListAPIs(ctx)
		if err != nil {
			return errMsg{Service: "api gateway", Err: err}
		}
		return apiListLoadedMsg(apis)
	}
}

func (m Model) loadLogGroups() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		groups, err := m.client.ListLogGroups(ctx)
		if err != nil {
			return errMsg{Service: "cloudwatch", Err: err}
		}
		return logGroupsLoadedMsg(groups)
	}
}

func (m Model) tailLogs() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		lines, err := m.client.TailLogSample(ctx)
		if err != nil {
			return errMsg{Service: "cloudwatch tail", Err: err}
		}
		return logLinesAppendedMsg(lines)
	}
}

func (m Model) loadDistributions() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		distributions, err := m.client.ListDistributions(ctx)
		if err != nil {
			return errMsg{Service: "cloudfront", Err: err}
		}
		return distributionsLoadedMsg(distributions)
	}
}

var styles = struct {
	sidebar lipgloss.Style
	content lipgloss.Style
	header  lipgloss.Style
	footer  lipgloss.Style
	error   lipgloss.Style
}{
	sidebar: lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), false, true, false, false).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1),
	content: lipgloss.NewStyle().
		Padding(0, 1),
	header: lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		MarginBottom(1),
	footer: lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		MarginTop(1),
	error: lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")),
}
