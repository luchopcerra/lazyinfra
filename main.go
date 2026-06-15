package main

import (
	"context"
	"fmt"
	"os"

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
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("lazyinfra %s (%s, %s)\n", version, commit, date)
		return
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
