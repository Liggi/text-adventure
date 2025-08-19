package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sashabaranov/go-openai"
)

type llmResponseMsg struct {
	response string
	err      error
}

type model struct {
	messages []string
	input    string
	cursor   int
	width    int
	height   int
	client   *openai.Client
	debug    bool
	loading  bool
}

func initialModel() model {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("Please set OPENAI_API_KEY environment variable")
		os.Exit(1)
	}
	
	client := openai.NewClient(apiKey)
	debugMode := os.Getenv("DEBUG") == "1" || os.Getenv("DEBUG") == "true"
	
	if debugMode {
		logFile, err := os.OpenFile("debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			log.SetOutput(logFile)
		}
	}
	
	return model{
		messages: []string{},
		input:    "",
		cursor:   0,
		client:   client,
		debug:    debugMode,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m *model) debugLog(msg string) {
	if m.debug {
		log.Println(msg)
		m.messages = append(m.messages, "[DEBUG] "+msg)
	}
}

func callLLMAsync(client *openai.Client, userInput string, debug bool) tea.Cmd {
	return func() tea.Msg {
		if debug {
			log.Printf("Calling LLM with input: %q", userInput)
		}
		
		resp, err := client.CreateChatCompletion(
			context.Background(),
			openai.ChatCompletionRequest{
				Model: "gpt-5-2025-08-07",
				Messages: []openai.ChatCompletionMessage{
					{
						Role: openai.ChatMessageRoleSystem,
						Content: "You are a text adventure narrator. Create engaging, immersive responses to player actions. Keep responses concise but vivid. The player is exploring a mysterious world.",
					},
					{
						Role: openai.ChatMessageRoleUser,
						Content: userInput,
					},
				},
				MaxCompletionTokens: 200,
				ReasoningEffort:     "minimal",
			},
		)
		
		if err != nil {
			if debug {
				log.Printf("API error: %v", err)
			}
			return llmResponseMsg{response: "", err: err}
		}
		
		if debug {
			log.Printf("API response received, choices: %d", len(resp.Choices))
		}
		
		if len(resp.Choices) > 0 {
			response := strings.TrimSpace(resp.Choices[0].Message.Content)
			if debug {
				log.Printf("Response content: %q", response)
			}
			return llmResponseMsg{response: response, err: nil}
		}
		
		return llmResponseMsg{response: "The adventure continues...", err: nil}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case llmResponseMsg:
		if m.loading {
			m.messages = m.messages[:len(m.messages)-1]
			if msg.err != nil {
				m.messages = append(m.messages, fmt.Sprintf("Error: %v", msg.err))
			} else {
				m.messages = append(m.messages, msg.response)
			}
			m.messages = append(m.messages, "")
			m.loading = false
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "enter":
			if strings.TrimSpace(m.input) != "" && !m.loading {
				userInput := m.input
				m.messages = append(m.messages, "> "+userInput)
				m.input = ""
				m.loading = true
				m.messages = append(m.messages, "Thinking...")
				
				return m, callLLMAsync(m.client, userInput, m.debug)
			}
			return m, nil

		case "backspace":
			if len(m.input) > 0 && !m.loading {
				m.input = m.input[:len(m.input)-1]
			}
			return m, nil

		default:
			if len(msg.String()) == 1 && !m.loading {
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

	debugStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("11")).
		Padding(0, 1)

	loadingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Italic(true).
		Padding(0, 1)

	for _, message := range visibleMessages {
		if message == "" {
			chatContent.WriteString("\n")
		} else if strings.HasPrefix(message, "> ") {
			chatContent.WriteString(userStyle.Render(message) + "\n")
		} else if strings.HasPrefix(message, "[DEBUG] ") {
			chatContent.WriteString(debugStyle.Render(message) + "\n")
		} else if message == "Thinking..." {
			chatContent.WriteString(loadingStyle.Render(message) + "\n")
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