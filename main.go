package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	infraaws "lazyinfra/aws"
	"lazyinfra/ui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version":
			fmt.Printf("lazyinfra %s (%s, %s)\n", version, commit, date)
			return
		case "sso", "login":
			runSSO()
			return
		}
	}

	ctx := context.Background()

	profile := os.Getenv("AWS_PROFILE")
	isLocalStack := os.Getenv("LAZYINFRA_LOCALSTACK") == "1"

	client, err := infraaws.NewClient(ctx, profile, isLocalStack)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize AWS client: %v\n", err)
		os.Exit(1)
	}

	program := tea.NewProgram(ui.NewModel(client), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "lazyinfra failed: %v\n", err)
		os.Exit(1)
	}
}

func runSSO() {
	startURL := os.Getenv("SSO_START_URL")
	region := os.Getenv("SSO_REGION")
	if region == "" {
		region = "us-east-1"
	}

	if startURL == "" {
		fmt.Fprintf(os.Stderr, "SSO_START_URL environment variable is required\n")
		fmt.Fprintf(os.Stderr, "Example: SSO_START_URL=https://my-company.awsapps.com/start lazyinfra sso\n")
		os.Exit(1)
	}

	cfg := infraaws.SSOConfig{
		StartURL: startURL,
		Region:   region,
	}

	ctx := context.Background()
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("lazyinfra sso - AWS SSO Login")
	fmt.Println("=============================")
	fmt.Printf("Start URL: %s\n", cfg.StartURL)
	fmt.Printf("Region:    %s\n", cfg.Region)
	fmt.Println()

	deviceCode, userCode, verificationURI, verificationURIComplete,
		clientSecret, clientID, _, err := infraaws.DeviceAuthInfo(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "device auth failed: %v\n", err)
		os.Exit(1)
	}

	_ = infraaws.OpenBrowser(awsToString(verificationURIComplete))

	fmt.Println("Open this URL in your browser:")
	fmt.Printf("  %s\n\n", awsToString(verificationURI))
	fmt.Printf("Your one-time code: %s\n\n", awsToString(userCode))
	fmt.Print("Press Enter after completing the browser login...")
	_, _ = reader.ReadString('\n')

	fmt.Println("Polling for authentication...")
	token, err := infraaws.PollToken(ctx, cfg, clientID, clientSecret, deviceCode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "token poll failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Authentication successful!")
	fmt.Println()

	fmt.Println("Fetching AWS accounts...")
	accounts, err := infraaws.ListAccounts(ctx, cfg.Region, token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list accounts failed: %v\n", err)
		os.Exit(1)
	}
	if len(accounts) == 0 {
		fmt.Fprintf(os.Stderr, "no AWS accounts found\n")
		os.Exit(1)
	}

	fmt.Println("Select an AWS account:")
	for i, a := range accounts {
		fmt.Printf("  %2d. %-15s %s\n", i+1, a.AccountID, a.AccountName)
	}

	accountIdx := promptInt(reader, "Account number", 1, len(accounts)) - 1
	selectedAccount := accounts[accountIdx]
	fmt.Printf("Selected: %s (%s)\n", selectedAccount.AccountID, selectedAccount.AccountName)
	fmt.Println()

	fmt.Println("Fetching available roles...")
	roles, err := infraaws.ListAccountRoles(ctx, cfg.Region, token, selectedAccount.AccountID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list roles failed: %v\n", err)
		os.Exit(1)
	}
	if len(roles) == 0 {
		fmt.Fprintf(os.Stderr, "no roles found for account %s\n", selectedAccount.AccountID)
		os.Exit(1)
	}

	fmt.Println("Select an IAM role:")
	for i, r := range roles {
		fmt.Printf("  %2d. %s\n", i+1, r.RoleName)
	}

	roleIdx := promptInt(reader, "Role number", 1, len(roles)) - 1
	selectedRole := roles[roleIdx]
	fmt.Printf("Selected: %s\n", selectedRole.RoleName)
	fmt.Println()

	creds, err := infraaws.GetRoleCredentials(ctx, cfg.Region, token, selectedAccount.AccountID, selectedRole.RoleName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "get credentials failed: %v\n", err)
		os.Exit(1)
	}

	if err := infraaws.WriteCredentials("default", creds); err != nil {
		fmt.Fprintf(os.Stderr, "write credentials failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Credentials written to ~/.aws/credentials [default]")
	fmt.Printf("  Access Key: %s\n", maskKey(creds.AccessKeyID))
	fmt.Printf("  Expires:    %s\n", creds.Expiration.Format(time.RFC3339))
}

func awsToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func promptInt(reader *bufio.Reader, prompt string, min, max int) int {
	for {
		fmt.Printf("%s [%d-%d]: ", prompt, min, max)
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "read error: %v\n", err)
			os.Exit(1)
		}
		line = strings.TrimSpace(line)
		var n int
		if _, err := fmt.Sscanf(line, "%d", &n); err == nil && n >= min && n <= max {
			return n
		}
		fmt.Printf("Invalid input. Enter a number between %d and %d.\n", min, max)
	}
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return key
	}
	return key[:4] + "****" + key[len(key)-4:]
}
