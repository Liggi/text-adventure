package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func animationTimer() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return animationTickMsg{}
	})
}
