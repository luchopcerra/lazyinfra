package views

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	infraaws "lazyinfra/aws"
)

type SSOState int

const (
	SSOIdle SSOState = iota
	SSODeviceAuth
	SSOPolling
	SSOSelectAccount
	SSOSelectRole
	SSOSuccess
	SSOError
)

type CredentialsModel struct {
	state           SSOState
	startURL        string
	region          string
	userCode        string
	verificationURI string
	clientID        string
	clientSecret    string
	deviceCode      string
	accessToken     *string
	accounts        []infraaws.SSOAccount
	roles           []infraaws.SSORole
	selected        int
	creds           *infraaws.AWSCredentials
	errMsg          string
	lastUpdated     string
	width           int
	height          int
}

func NewCredentialsModel() CredentialsModel {
	return CredentialsModel{
		startURL: os.Getenv("SSO_START_URL"),
		region:   envOrDefault("SSO_REGION", "us-east-1"),
	}
}

func (m CredentialsModel) GetStartURL() string      { return m.startURL }
func (m CredentialsModel) GetRegion() string         { return m.region }
func (m CredentialsModel) State() SSOState            { return m.state }
func (m CredentialsModel) GetClientID() string        { return m.clientID }
func (m CredentialsModel) GetClientSecret() string     { return m.clientSecret }
func (m CredentialsModel) GetDeviceCode() string       { return m.deviceCode }

func (m *CredentialsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *CredentialsModel) SetAccounts(accounts []infraaws.SSOAccount) {
	m.accounts = accounts
	m.selected = 0
	m.state = SSOSelectAccount
	m.errMsg = ""
}

func (m *CredentialsModel) SetRoles(roles []infraaws.SSORole) {
	m.roles = roles
	m.selected = 0
	m.state = SSOSelectRole
	m.errMsg = ""
}

func (m *CredentialsModel) SetCredentials(creds *infraaws.AWSCredentials) {
	m.creds = creds
	m.state = SSOSuccess
	m.lastUpdated = time.Now().Format(time.RFC3339)
}

func (m *CredentialsModel) SetSSOConfig(startURL, region string) {
	m.startURL = startURL
	m.region = region
}

func (m *CredentialsModel) SetDeviceAuth(userCode, verificationURI, clientID, clientSecret, deviceCode string) {
	m.userCode = userCode
	m.verificationURI = verificationURI
	m.clientID = clientID
	m.clientSecret = clientSecret
	m.deviceCode = deviceCode
	m.state = SSODeviceAuth
	m.errMsg = ""
}

func (m *CredentialsModel) SetPolling() {
	m.state = SSOPolling
	m.errMsg = ""
}

func (m *CredentialsModel) SetToken(accessToken *string) {
	m.accessToken = accessToken
}

func (m *CredentialsModel) SetError(errMsg string) {
	m.errMsg = errMsg
	m.state = SSOError
}

func (m *CredentialsModel) Reset() {
	m.state = SSOIdle
	m.userCode = ""
	m.verificationURI = ""
	m.clientID = ""
	m.clientSecret = ""
	m.deviceCode = ""
	m.accessToken = nil
	m.accounts = nil
	m.roles = nil
	m.selected = 0
	m.creds = nil
	m.errMsg = ""
}

func (m *CredentialsModel) SelectedAccount() *infraaws.SSOAccount {
	if m.selected < 0 || m.selected >= len(m.accounts) {
		return nil
	}
	return &m.accounts[m.selected]
}

func (m *CredentialsModel) SelectedRole() *infraaws.SSORole {
	if m.selected < 0 || m.selected >= len(m.roles) {
		return nil
	}
	return &m.roles[m.selected]
}

func (m *CredentialsModel) Update(msg tea.Msg) tea.Cmd {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	switch key.String() {
	case "up", "k":
		if m.state == SSOSelectAccount && len(m.accounts) > 0 {
			m.selected = max(0, m.selected-1)
		} else if m.state == SSOSelectRole && len(m.roles) > 0 {
			m.selected = max(0, m.selected-1)
		}
	case "down", "j":
		if m.state == SSOSelectAccount && len(m.accounts) > 0 {
			m.selected = min(len(m.accounts)-1, m.selected+1)
		} else if m.state == SSOSelectRole && len(m.roles) > 0 {
			m.selected = min(len(m.roles)-1, m.selected+1)
		}
	}

	return nil
}

func (m CredentialsModel) View() string {
	var b strings.Builder

	b.WriteString(sectionTitle.Render("SSO Credentials"))
	b.WriteString("\n\n")

	switch m.state {
	case SSOIdle:
		m.renderIdle(&b)
	case SSODeviceAuth:
		m.renderDeviceAuth(&b)
	case SSOPolling:
		m.renderPolling(&b)
	case SSOSelectAccount:
		m.renderAccountList(&b)
	case SSOSelectRole:
		m.renderRoleList(&b)
	case SSOSuccess:
		m.renderSuccess(&b)
	case SSOError:
		m.renderError(&b)
	}

	return lipgloss.NewStyle().Width(m.width).Render(b.String())
}

func (m CredentialsModel) renderIdle(b *strings.Builder) {
	if m.creds != nil && !m.creds.Expiration.IsZero() && time.Until(m.creds.Expiration) > 0 {
		b.WriteString(ok.Render("Credentials active"))
		b.WriteString(fmt.Sprintf("\n  Expires: %s", m.creds.Expiration.Format(time.RFC3339)))
		if m.lastUpdated != "" {
			b.WriteString(fmt.Sprintf("\n  Updated: %s", m.lastUpdated))
		}
		b.WriteString("\n\n")
	} else {
		b.WriteString(muted.Render("No active credentials"))
		b.WriteString("\n\n")
	}

	b.WriteString(panel.Render(
		"With SSO credentials, lazyinfra will:\n" +
			"  1. Open your browser for SSO login\n" +
			"  2. Fetch your AWS accounts and roles\n" +
			"  3. Let you pick an account and role\n" +
			"  4. Write credentials to ~/.aws/credentials\n",
	))
	b.WriteString("\n")

	if m.startURL == "" {
		b.WriteString(warn.Render("SSO_START_URL not set"))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  %s\n", muted.Render("Set SSO_START_URL env var (e.g. https://my-company.awsapps.com/start)")))
		b.WriteString("\n")
	} else {
		b.WriteString(fmt.Sprintf("  Start URL: %s\n", m.startURL))
		b.WriteString(fmt.Sprintf("  Region:     %s\n", m.region))
		b.WriteString("\n")
	}

	if m.errMsg != "" {
		b.WriteString(errorLine.Render(fmt.Sprintf("  Last error: %s", m.errMsg)))
		b.WriteString("\n\n")
	}

	b.WriteString(muted.Render("Press l to start SSO login"))
}

func (m CredentialsModel) renderDeviceAuth(b *strings.Builder) {
	b.WriteString(sectionTitle.Render("Step 1: Authenticate in Browser"))
	b.WriteString("\n\n")

	b.WriteString(panel.Render(fmt.Sprintf(
		"Open this URL in your browser:\n\n%s\n\nYour one-time code: %s",
		selected.Render(m.verificationURI),
		badge.Render(m.userCode),
	)))
	b.WriteString("\n\n")

	b.WriteString(muted.Render("Press p to start polling for authentication"))
}

func (m CredentialsModel) renderPolling(b *strings.Builder) {
	b.WriteString(sectionTitle.Render("Step 2: Waiting for Authentication"))
	b.WriteString("\n\n")

	b.WriteString(panel.Render(
		"Waiting for you to complete the browser login...\n" +
			"  Press esc to cancel",
	))
}

func (m CredentialsModel) renderAccountList(b *strings.Builder) {
	b.WriteString(sectionTitle.Render("Step 3: Select AWS Account"))
	b.WriteString("\n\n")

	if len(m.accounts) == 0 {
		b.WriteString(muted.Render("No accounts found"))
		return
	}

	for i, a := range m.accounts {
		line := fmt.Sprintf("%-15s %s", a.AccountID, a.AccountName)
		if i == m.selected {
			b.WriteString(selected.Render(line) + "\n")
		} else {
			b.WriteString(line + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(muted.Render("Use up/down to navigate, enter to select, esc to cancel"))
}

func (m CredentialsModel) renderRoleList(b *strings.Builder) {
	b.WriteString(sectionTitle.Render("Step 4: Select IAM Role"))
	b.WriteString("\n\n")

	if len(m.roles) == 0 {
		b.WriteString(muted.Render("No roles found for this account"))
		return
	}

	for i, r := range m.roles {
		line := r.RoleName
		if i == m.selected {
			b.WriteString(selected.Render(line) + "\n")
		} else {
			b.WriteString(line + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(muted.Render("Use up/down to navigate, enter to select, esc to cancel"))
}

func (m CredentialsModel) renderSuccess(b *strings.Builder) {
	b.WriteString(ok.Render("Credentials Updated Successfully"))
	b.WriteString("\n\n")

	if m.creds != nil {
		b.WriteString(fmt.Sprintf("  Access Key:     %s\n", maskKey(m.creds.AccessKeyID)))
		b.WriteString(fmt.Sprintf("  Expiration:     %s\n", m.creds.Expiration.Format(time.RFC3339)))
		b.WriteString(fmt.Sprintf("  Profile:        %s\n", badge.Render("default")))
	}

	b.WriteString("\n")
	b.WriteString(muted.Render("Press l to login again, or r to refresh"))
}

func (m CredentialsModel) renderError(b *strings.Builder) {
	b.WriteString(errorLine.Render("Error"))
	b.WriteString("\n\n")
	b.WriteString(panel.Render(m.errMsg))
	b.WriteString("\n\n")
	b.WriteString(muted.Render("Press l to try again"))
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return key
	}
	return key[:4] + "****" + key[len(key)-4:]
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
