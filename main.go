package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	messages []string
	input    string
	cursor   int
	width    int
	height   int
}

func initialModel() model {
	return model{
		messages: []string{},
		input:    "",
		cursor:   0,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "enter":
			if strings.TrimSpace(m.input) != "" {
				m.messages = append(m.messages, "> "+m.input)
				m.messages = append(m.messages, "You said: "+m.input)
				m.messages = append(m.messages, "")
				m.input = ""
			}
			return m, nil

		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
			return m, nil

		default:
			if len(msg.String()) == 1 {
				m.input += msg.String()
			}
			return m, nil
		}
	}

	return m, nil
}

func (m model) View() string {
	inputHeight := 3
	chatHeight := m.height - inputHeight
	rightWidth := m.width

	messageStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7")).
		Padding(0, 1)

	userStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true).
		Padding(0, 1)

	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")).
		Padding(0, 1).
		Width(m.width - 4)

	chatPanel := lipgloss.NewStyle().
		Width(rightWidth).
		Height(chatHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")).
		Padding(1)

	var chatContent strings.Builder
	
	visibleMessages := m.messages
	maxMessages := chatHeight - 2
	if maxMessages < 1 {
		maxMessages = 1
	}
	
	if len(visibleMessages) > maxMessages {
		visibleMessages = visibleMessages[len(visibleMessages)-maxMessages:]
	}

	paddingLines := maxMessages - len(visibleMessages)
	if paddingLines > 0 {
		for i := 0; i < paddingLines; i++ {
			chatContent.WriteString("\n")
		}
	}

	for _, message := range visibleMessages {
		if message == "" {
			chatContent.WriteString("\n")
		} else if strings.HasPrefix(message, "> ") {
			chatContent.WriteString(userStyle.Render(message) + "\n")
		} else {
			chatContent.WriteString(messageStyle.Render(message) + "\n")
		}
	}

	chat := chatPanel.Render(chatContent.String())
	input := inputStyle.Render(m.input + "│")

	return chat + "\n" + input
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
	}
}