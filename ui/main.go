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
	serviceCredentials
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
	{serviceCredentials, "SSO Credentials", "login and update default profile"},
}

type Model struct {
	client *infraaws.AWSClient

	sidebar list.Model
	active  service
	width   int
	height  int

	api         views.APIModel
	lambda      views.LambdaModel
	cloudwatch  views.CloudWatchModel
	cloudfront  views.CloudFrontModel
	credentials views.CredentialsModel
	lastErr     error

	tailGroup  string
	tailEvents <-chan infraaws.TailEvent
	tailCancel context.CancelFunc

	accessToken *string
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
		client:      client,
		sidebar:     sidebar,
		active:      serviceAPIGateway,
		api:         views.NewAPIModel(),
		lambda:      views.NewLambdaModel(),
		cloudwatch:  views.NewCloudWatchModel(),
		cloudfront:  views.NewCloudFrontModel(),
		credentials: views.NewCredentialsModel(),
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
		m.credentials.SetSize(contentWidth, msg.Height-2)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.tailCancel != nil {
				m.tailCancel()
			}
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
		case "5":
			m.sidebar.Select(4)
			m.active = serviceCredentials
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
				cmds = append(cmds, m.startLogTail())
			}
		case "i":
			if m.active == serviceCloudFront && !m.cloudfront.IsEditingPath() {
				m.cloudfront.SetStatus("Creating invalidation...")
				cmds = append(cmds, m.createInvalidation())
			}
		case "l":
			if m.active == serviceCredentials && m.credentials.State() == views.SSOError {
				m.credentials.Reset()
			}
			if m.active == serviceCredentials && (m.credentials.State() == views.SSOIdle || m.credentials.State() == views.SSOSuccess) {
				if !m.credentials.IsValid() {
					m.credentials.SetError("configure a Start URL and Region first")
				} else {
					m.credentials.SetError("")
					cmds = append(cmds, m.startSSODeviceAuth())
				}
			}
		case "c":
			if m.active == serviceCredentials && m.credentials.State() == views.SSOIdle && !m.credentials.IsValid() {
				m.credentials.StartConfiguring()
			}
		case "p":
			if m.active == serviceCredentials && m.credentials.State() == views.SSODeviceAuth {
				m.credentials.SetPolling()
				cmds = append(cmds, m.pollSSOToken())
			}
		}
	case errMsg:
		m.lastErr = fmt.Errorf("%s: %w", msg.Service, msg.Err)
		if msg.Service == "cloudwatch tail" {
			m.cloudwatch.SetTailing(false)
		}
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
		cmds = append(cmds, m.waitForLogEvent())
	case logTailStartedMsg:
		if m.tailCancel != nil {
			m.tailCancel()
		}
		m.lastErr = nil
		m.tailGroup = msg.Group
		m.tailEvents = msg.Events
		m.tailCancel = msg.Cancel
		m.cloudwatch.SetTailing(true)
		m.cloudwatch.AppendLines([]string{fmt.Sprintf("tailing %s", msg.Group)})
		cmds = append(cmds, m.waitForLogEvent())
	case distributionsLoadedMsg:
		m.lastErr = nil
		m.cloudfront.SetDistributions([]infraaws.Distribution(msg))
	case invalidationCreatedMsg:
		m.lastErr = nil
		result := infraaws.InvalidationResult(msg)
		m.cloudfront.SetStatus(fmt.Sprintf("Invalidation %s created for %s (%s)", result.ID, result.Path, result.Status))
	case ssoDeviceAuthMsg:
		if msg.Err != nil {
			m.lastErr = msg.Err
			m.credentials.SetError(msg.Err.Error())
		} else {
			m.lastErr = nil
			m.credentials.SetDeviceAuth(msg.UserCode, msg.VerificationURI, msg.ClientID, msg.ClientSecret, msg.DeviceCode)
		}
	case ssoTokenMsg:
		if msg.Err != nil {
			m.lastErr = msg.Err
			m.credentials.SetError(msg.Err.Error())
		} else {
			m.lastErr = nil
			m.accessToken = msg.AccessToken
			cmds = append(cmds, m.loadSSOAccounts())
		}
	case ssoAccountsLoadedMsg:
		if msg.Err != nil {
			m.lastErr = msg.Err
			m.credentials.SetError(msg.Err.Error())
		} else {
			m.lastErr = nil
			m.credentials.SetAccounts(msg.Accounts)
		}
	case ssoRolesLoadedMsg:
		if msg.Err != nil {
			m.lastErr = msg.Err
			m.credentials.SetError(msg.Err.Error())
		} else {
			m.lastErr = nil
			m.credentials.SetRoles(msg.Roles)
		}
	case ssoCredentialsMsg:
		if msg.Err != nil {
			m.lastErr = msg.Err
			m.credentials.SetError(msg.Err.Error())
		} else {
			m.lastErr = nil
			m.credentials.SetCredentials(msg.Credentials)
		}
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
	case serviceCredentials:
		cmds = append(cmds, m.credentials.Update(msg))
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
	case serviceCredentials:
		body = m.credentials.View()
	}

	footerParts := []string{"1-5 switch", "enter select", "r refresh", "q quit"}
	if m.active == serviceCloudWatch {
		footerParts = append(footerParts, "t tail selected group")
	}
	if m.active == serviceCloudFront {
		footerParts = append(footerParts, "e edit path", "esc done editing", "i create invalidation")
	}
	if m.active == serviceCredentials {
		if m.credentials.IsConfiguring() {
			footerParts = append(footerParts, "tab switch", "enter confirm", "esc cancel")
		} else {
			if !m.credentials.IsValid() {
				footerParts = append(footerParts, "c configure")
			} else {
				footerParts = append(footerParts, "l login")
			}
			footerParts = append(footerParts, "p poll")
		}
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

func (m Model) startLogTail() tea.Cmd {
	logGroup := m.cloudwatch.SelectedLogGroup()
	return func() tea.Msg {
		if logGroup == "" {
			return errMsg{Service: "cloudwatch tail", Err: fmt.Errorf("select a log group before tailing")}
		}
		ctx, cancel := context.WithCancel(context.Background())
		events := make(chan infraaws.TailEvent, 256)
		go m.client.TailLogGroup(ctx, logGroup, events)
		return logTailStartedMsg{Group: logGroup, Events: events, Cancel: cancel}
	}
}

func (m Model) waitForLogEvent() tea.Cmd {
	events := m.tailEvents
	if events == nil {
		return nil
	}

	return func() tea.Msg {
		event, ok := <-events
		if !ok {
			return errMsg{Service: "cloudwatch tail", Err: fmt.Errorf("tail stream stopped")}
		}
		if event.Err != nil {
			return errMsg{Service: "cloudwatch tail", Err: event.Err}
		}
		if event.Line == "" {
			return nil
		}
		return logLinesAppendedMsg([]string{event.Line})
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

func (m Model) createInvalidation() tea.Cmd {
	distributionID := m.cloudfront.SelectedDistributionID()
	invalidationPath := m.cloudfront.InvalidationPath()

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := m.client.CreateInvalidation(ctx, distributionID, invalidationPath)
		if err != nil {
			return errMsg{Service: "cloudfront invalidation", Err: err}
		}
		return invalidationCreatedMsg(result)
	}
}

func (m Model) ssoCfg() infraaws.SSOConfig {
	return infraaws.SSOConfig{
		StartURL: m.credentials.GetStartURL(),
		Region:   m.credentials.GetRegion(),
	}
}

func (m Model) startSSODeviceAuth() tea.Cmd {
	cfg := m.ssoCfg()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		deviceCode, userCode, verificationURI, verificationURIComplete,
			clientSecret, clientID, _, err := infraaws.DeviceAuthInfo(ctx, cfg)
		if err != nil {
			return ssoDeviceAuthMsg{Err: err}
		}

		_ = infraaws.OpenBrowser(awsToString(verificationURIComplete))

		return ssoDeviceAuthMsg{
			UserCode:        awsToString(userCode),
			VerificationURI: awsToString(verificationURI),
			ClientID:        awsToString(clientID),
			ClientSecret:    awsToString(clientSecret),
			DeviceCode:      awsToString(deviceCode),
		}
	}
}

func (m Model) pollSSOToken() tea.Cmd {
	cfg := m.ssoCfg()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		token, err := infraaws.PollToken(ctx, cfg,
			awsString(m.credentials.GetClientID()),
			awsString(m.credentials.GetClientSecret()),
			awsString(m.credentials.GetDeviceCode()),
		)
		if err != nil {
			return ssoTokenMsg{Err: err}
		}
		return ssoTokenMsg{AccessToken: token}
	}
}

func (m Model) loadSSOAccounts() tea.Cmd {
	cfg := m.ssoCfg()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		accounts, err := infraaws.ListAccounts(ctx, cfg.Region, m.accessToken)
		if err != nil {
			return ssoAccountsLoadedMsg{Err: err}
		}
		return ssoAccountsLoadedMsg{Accounts: accounts}
	}
}

func (m Model) loadSSORoles() tea.Cmd {
	account := m.credentials.SelectedAccount()
	cfg := m.ssoCfg()
	return func() tea.Msg {
		if account == nil {
			return ssoRolesLoadedMsg{Err: fmt.Errorf("no account selected")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		roles, err := infraaws.ListAccountRoles(ctx, cfg.Region, m.accessToken, account.AccountID)
		if err != nil {
			return ssoRolesLoadedMsg{Err: err}
		}
		return ssoRolesLoadedMsg{Roles: roles}
	}
}

func (m Model) fetchSSOCredentials() tea.Cmd {
	account := m.credentials.SelectedAccount()
	role := m.credentials.SelectedRole()
	cfg := m.ssoCfg()
	return func() tea.Msg {
		if account == nil || role == nil {
			return ssoCredentialsMsg{Err: fmt.Errorf("no account or role selected")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		creds, err := infraaws.GetRoleCredentials(ctx, cfg.Region, m.accessToken, account.AccountID, role.RoleName)
		if err != nil {
			return ssoCredentialsMsg{Err: err}
		}

		if err := infraaws.WriteCredentials("default", creds); err != nil {
			return ssoCredentialsMsg{Err: fmt.Errorf("write credentials: %w", err)}
		}

		return ssoCredentialsMsg{Credentials: creds}
	}
}

func awsToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func awsString(s string) *string {
	return &s
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
