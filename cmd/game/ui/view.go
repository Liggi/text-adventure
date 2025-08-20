package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	inputHeight := 3
	chatHeight := m.height - inputHeight
	rightWidth := m.width

	messageStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7"))

	userStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true)

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

	debugStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("11"))

	loadingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("6"))

	contentWidth := rightWidth - 4
	
	for _, message := range visibleMessages {
		if message == "" {
			chatContent.WriteString("\n")
		} else if strings.HasPrefix(message, "> ") {
			wrappedText := wrapAndIndent(message, contentWidth, " ")
			chatContent.WriteString(userStyle.Render(wrappedText) + "\n")
		} else if strings.HasPrefix(message, "[DEBUG] ") {
			wrappedText := wrapAndIndent(message, contentWidth, " ")
			chatContent.WriteString(debugStyle.Render(wrappedText) + "\n")
		} else if message == "LOADING_ANIMATION" {
			animationText := getLoadingAnimation(m.animationFrame)
			wrappedText := wrapAndIndent(animationText, contentWidth, " ")
			chatContent.WriteString(loadingStyle.Render(wrappedText) + "\n")
		} else {
			wrappedText := wrapAndIndent(message, contentWidth, " ")
			chatContent.WriteString(messageStyle.Render(wrappedText) + "\n")
		}
	}

	chat := chatPanel.Render(chatContent.String())
	input := inputStyle.Render(m.input + "│")

	return chat + "\n" + input
}

func wrapAndIndent(text string, width int, indent string) string {
	if len(text) <= width {
		return indent + text
	}
	
	var result strings.Builder
	words := strings.Fields(text)
	if len(words) == 0 {
		return indent + text
	}
	
	currentLine := indent + words[0]
	
	for _, word := range words[1:] {
		if len(currentLine)+1+len(word) <= width {
			currentLine += " " + word
		} else {
			result.WriteString(currentLine + "\n")
			currentLine = indent + word
		}
	}
	
	result.WriteString(currentLine)
	return result.String()
}

func getLoadingAnimation(frame int) string {
	arc := []string{"◜", "◠", "◝", "◞", "◡", "◟"}
	return arc[frame%len(arc)]
}