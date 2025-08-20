package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	model, err := createApp()
	if err != nil {
		fmt.Printf("Error initializing app: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running app: %v\n", err)
		os.Exit(1)
	}
}