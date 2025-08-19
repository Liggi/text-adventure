// Text Adventure Game with GPT-5 Integration
// This is a terminal-based text adventure game that uses OpenAI's GPT-5 model
// to generate dynamic responses to player actions. Built with Bubble Tea TUI framework.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"        // Terminal UI framework for Go
	"github.com/charmbracelet/lipgloss"             // Styling library for terminal UIs
	"github.com/sashabaranov/go-openai"             // OpenAI API client
)

type llmResponseMsg struct {
	response string
	err      error
}

type llmStreamChunkMsg struct {
	chunk  string
	stream *openai.ChatCompletionStream
	debug  bool
}

type llmStreamCompleteMsg struct{}

type model struct {
	messages       []string
	input          string
	cursor         int
	width          int
	height         int
	client         *openai.Client
	debug          bool
	loading        bool
	streaming      bool
	currentResponse string
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

func startLLMStream(client *openai.Client, userInput string, debug bool) tea.Cmd {
	return func() tea.Msg {
		if debug {
			log.Printf("Starting LLM stream with input: %q", userInput)
		}
		
		req := openai.ChatCompletionRequest{
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
			Stream:              true,
		}
		
		stream, err := client.CreateChatCompletionStream(context.Background(), req)
		if err != nil {
			if debug {
				log.Printf("Stream creation error: %v", err)
			}
			return llmResponseMsg{response: "", err: err}
		}
		
		// Store stream in a way we can access it from readNextChunk
		// For simplicity, we'll create a streaming state message
		return streamStartedMsg{stream: stream, debug: debug}
	}
}

type streamStartedMsg struct {
	stream *openai.ChatCompletionStream
	debug  bool
}

func readNextChunk(stream *openai.ChatCompletionStream, debug bool) tea.Cmd {
	return func() tea.Msg {
		response, err := stream.Recv()
		
		if errors.Is(err, io.EOF) {
			if debug {
				log.Println("Stream finished")
			}
			stream.Close()
			return llmStreamCompleteMsg{}
		}
		
		if err != nil {
			if debug {
				log.Printf("Stream error: %v", err)
			}
			stream.Close()
			return llmResponseMsg{response: "", err: err}
		}
		
		if len(response.Choices) > 0 && response.Choices[0].Delta.Content != "" {
			chunk := response.Choices[0].Delta.Content
			if debug {
				log.Printf("Stream chunk: %q", chunk)
			}
			return llmStreamChunkMsg{chunk: chunk, stream: stream, debug: debug}
		}
		
		// Empty chunk, keep reading
		return readNextChunk(stream, debug)()
	}
}

func callLLMAsync(client *openai.Client, userInput string, debug bool) tea.Cmd {
	return func() tea.Msg {
		if debug {
			log.Printf("Starting LLM async call with input: %q", userInput)
		}
		
		req := openai.ChatCompletionRequest{
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
			Stream:              false,
		}
		
		response, err := client.CreateChatCompletion(context.Background(), req)
		if err != nil {
			if debug {
				log.Printf("LLM error: %v", err)
			}
			return llmResponseMsg{response: "", err: err}
		}
		
		if len(response.Choices) > 0 {
			content := response.Choices[0].Message.Content
			if debug {
				log.Printf("LLM response: %q", content)
			}
			return llmResponseMsg{response: content, err: nil}
		}
		
		return llmResponseMsg{response: "", err: fmt.Errorf("no response from LLM")}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case streamStartedMsg:
		if m.loading {
			m.messages = m.messages[:len(m.messages)-1]
			m.streaming = true
			m.currentResponse = ""
			m.messages = append(m.messages, "")
		}
		return m, readNextChunk(msg.stream, msg.debug)

	case llmStreamChunkMsg:
		if m.streaming {
			m.currentResponse += msg.chunk
			if len(m.messages) > 0 {
				m.messages[len(m.messages)-1] = m.currentResponse
			}
		}
		return m, readNextChunk(msg.stream, msg.debug)

	case llmStreamCompleteMsg:
		if m.streaming {
			m.streaming = false
			m.loading = false
			m.messages = append(m.messages, "")
		}
		return m, nil

	case llmResponseMsg:
		if m.loading && !m.streaming {
			m.messages = m.messages[:len(m.messages)-1]
			if msg.err != nil {
				m.messages = append(m.messages, fmt.Sprintf("Error: %v", msg.err))
			} else {
				m.messages = append(m.messages, msg.response)
			}
			m.messages = append(m.messages, "")
			m.loading = false
		} else if m.streaming {
			m.streaming = false
			m.loading = false
			if msg.err != nil {
				if len(m.messages) > 0 {
					m.messages[len(m.messages)-1] = fmt.Sprintf("Error: %v", msg.err)
				}
				m.messages = append(m.messages, "")
			}
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
				
				// Use streaming by default - change to callLLMAsync for non-streaming
				return m, startLLMStream(m.client, userInput, m.debug)
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

func (m model) View() string {
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
		Foreground(lipgloss.Color("8")).
		Italic(true)

	contentWidth := rightWidth - 4 // Account for border and padding
	
	for _, message := range visibleMessages {
		if message == "" {
			chatContent.WriteString("\n")
		} else if strings.HasPrefix(message, "> ") {
			wrappedText := wrapAndIndent(message, contentWidth, " ")
			chatContent.WriteString(userStyle.Render(wrappedText) + "\n")
		} else if strings.HasPrefix(message, "[DEBUG] ") {
			wrappedText := wrapAndIndent(message, contentWidth, " ")
			chatContent.WriteString(debugStyle.Render(wrappedText) + "\n")
		} else if message == "Thinking..." {
			wrappedText := wrapAndIndent(message, contentWidth, " ")
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

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
	}
}