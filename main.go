package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	infraaws "lazyinfra/aws"
	"lazyinfra/ui"
)

func main() {
	ctx := context.Background()

	client, err := infraaws.NewClient(ctx, infraaws.ConfigFromEnv())
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
