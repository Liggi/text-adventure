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
		messages: []string{"Welcome to the text adventure! Type something to begin..."},
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
	var s strings.Builder

	chatHeight := m.height - 4
	if chatHeight < 1 {
		chatHeight = 10
	}

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

	s.WriteString("\n")

	visibleMessages := m.messages
	if len(visibleMessages) > chatHeight {
		visibleMessages = visibleMessages[len(visibleMessages)-chatHeight:]
	}

	for _, message := range visibleMessages {
		if strings.HasPrefix(message, "> ") {
			s.WriteString(userStyle.Render(message) + "\n")
		} else {
			s.WriteString(messageStyle.Render(message) + "\n")
		}
	}

	s.WriteString("\n")
	s.WriteString(inputStyle.Render(m.input + "│"))
	s.WriteString("\n\n")
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Press 'q' or Ctrl+C to quit"))

	return s.String()
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
	}
}