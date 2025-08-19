// Text Adventure Game with GPT-5 Integration
// This is a terminal-based text adventure game that uses OpenAI's GPT-5 model
// to generate dynamic responses to player actions. Built with Bubble Tea TUI framework.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sashabaranov/go-openai"
)

type llmResponseMsg struct {
	response string
	err      error
}

type llmStreamChunkMsg struct {
	chunk         string
	stream        *openai.ChatCompletionStream
	debug         bool
	completionCtx *streamStartedMsg
}

type llmStreamCompleteMsg struct {
	world        WorldState
	userInput    string
	systemPrompt string
	response     string
	startTime    time.Time
	logger       *CompletionLogger
	debug        bool
}

type animationTickMsg struct{}

type WorldState struct {
	Location  string
	Inventory []string
	Locations map[string]LocationInfo
}

type LocationInfo struct {
	Title       string
	Description string
	Items       []string
	Exits       map[string]string
}

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
	animationFrame int
	world          WorldState
	gameHistory    []string // Last few exchanges for LLM context
	logger         *CompletionLogger
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
	
	logger, err := NewCompletionLogger()
	if err != nil {
		fmt.Printf("Failed to initialize completion logger: %v\n", err)
		os.Exit(1)
	}
	
	world := WorldState{
		Location:  "foyer",
		Inventory: []string{},
		Locations: map[string]LocationInfo{
			"foyer": {
				Title:       "Old Foyer",
				Description: "A dusty foyer with motes drifting in shafts of light",
				Items:       []string{"silver key"},
				Exits:       map[string]string{"north": "study"},
			},
			"study": {
				Title:       "Quiet Study",
				Description: "A quiet study with a heavy oak desk",
				Items:       []string{},
				Exits:       map[string]string{"south": "foyer"},
			},
		},
	}

	return model{
		messages:    []string{},
		input:       "",
		cursor:      0,
		client:      client,
		debug:       debugMode,
		world:       world,
		gameHistory: []string{},
		logger:      logger,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func animationTimer() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return animationTickMsg{}
	})
}

func getLoadingAnimation(frame int) string {
	// "arc" spinner from cli-spinners - battle-tested smooth circular motion
	arc := []string{"◜", "◠", "◝", "◞", "◡", "◟"}
	return arc[frame%len(arc)]
}

func buildWorldContext(world WorldState, gameHistory []string) string {
	currentLoc := world.Locations[world.Location]
	context := fmt.Sprintf(`WORLD STATE:
Current Location: %s (%s)
%s

Available Items Here: %v
Available Exits: %v
Player Inventory: %v

`, currentLoc.Title, world.Location, currentLoc.Description, currentLoc.Items, currentLoc.Exits, world.Inventory)

	if len(gameHistory) > 0 {
		context += "RECENT CONVERSATION:\n"
		for _, exchange := range gameHistory {
			context += exchange + "\n"
		}
		context += "\n"
	}

	return context
}

func (m *model) debugLog(msg string) {
	if m.debug {
		log.Println(msg)
		m.messages = append(m.messages, "[DEBUG] "+msg)
	}
}

func startLLMStream(client *openai.Client, userInput string, world WorldState, gameHistory []string, logger *CompletionLogger, debug bool) tea.Cmd {
	return func() tea.Msg {
		if debug {
			log.Printf("Starting LLM stream with input: %q", userInput)
		}
		
		startTime := time.Now()
		worldContext := buildWorldContext(world, gameHistory)
		systemPrompt := `You are both narrator and world simulator for a text adventure game. You have complete knowledge of the world state.

Your job: Respond to player actions with 2-4 sentence vivid narration that feels natural and immersive.

Rules:
- Stay consistent with the provided world state
- If action is impossible, explain why and suggest alternatives
- Keep responses concise but atmospheric
- Don't change the world state (that comes later)
- Respond as if you can see everything in the current location`

		req := openai.ChatCompletionRequest{
			Model: "gpt-5-2025-08-07",
			Messages: []openai.ChatCompletionMessage{
				{
					Role: openai.ChatMessageRoleSystem,
					Content: systemPrompt,
				},
				{
					Role: openai.ChatMessageRoleUser,
					Content: worldContext + "PLAYER ACTION: " + userInput,
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
		
		return streamStartedMsg{
			stream:       stream,
			debug:        debug,
			world:        world,
			userInput:    userInput,
			systemPrompt: systemPrompt,
			startTime:    startTime,
			logger:       logger,
		}
	}
}

type streamStartedMsg struct {
	stream       *openai.ChatCompletionStream
	debug        bool
	world        WorldState
	userInput    string
	systemPrompt string
	startTime    time.Time
	logger       *CompletionLogger
}

func readNextChunk(stream *openai.ChatCompletionStream, debug bool, completionCtx *streamStartedMsg, fullResponse string) tea.Cmd {
	return func() tea.Msg {
		response, err := stream.Recv()
		
		if errors.Is(err, io.EOF) {
			if debug {
				log.Println("Stream finished")
			}
			stream.Close()
			
			responseTime := time.Since(completionCtx.startTime)
			metadata := CompletionMetadata{
				Model:         "gpt-5-2025-08-07",
				MaxTokens:     200,
				ResponseTime:  responseTime,
				StreamingUsed: true,
			}
			
			// Best effort logging - don't fail if logging fails
			if logErr := completionCtx.logger.LogCompletion(completionCtx.world, completionCtx.userInput, completionCtx.systemPrompt, fullResponse, metadata); logErr != nil && debug {
				log.Printf("Failed to log completion: %v", logErr)
			}
			
			return llmStreamCompleteMsg{
				world:        completionCtx.world,
				userInput:    completionCtx.userInput,
				systemPrompt: completionCtx.systemPrompt,
				response:     fullResponse,
				startTime:    completionCtx.startTime,
				logger:       completionCtx.logger,
				debug:        debug,
			}
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
			return llmStreamChunkMsg{chunk: chunk, stream: stream, debug: debug, completionCtx: completionCtx}
		}
		
		// Empty chunk, keep reading
		return readNextChunk(stream, debug, completionCtx, fullResponse)()
	}
}


func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case animationTickMsg:
		if m.loading {
			m.animationFrame++
			return m, animationTimer()
		}
		return m, nil

	case streamStartedMsg:
		if m.loading {
			m.messages = m.messages[:len(m.messages)-1]
			m.streaming = true
			m.currentResponse = ""
			m.messages = append(m.messages, "")
		}
		return m, readNextChunk(msg.stream, msg.debug, &msg, "")

	case llmStreamChunkMsg:
		if m.streaming {
			m.currentResponse += msg.chunk
			if len(m.messages) > 0 {
				m.messages[len(m.messages)-1] = m.currentResponse
			}
		}
		return m, readNextChunk(msg.stream, msg.debug, msg.completionCtx, m.currentResponse)

	case llmStreamCompleteMsg:
		if m.streaming {
			m.streaming = false
			m.loading = false
			
			// Add the complete response to game history
			if len(m.messages) > 0 && m.currentResponse != "" {
				m.gameHistory = append(m.gameHistory, "Narrator: "+m.currentResponse)
				// Keep only last 3 exchanges (6 entries: player + narrator pairs)
				if len(m.gameHistory) > 6 {
					m.gameHistory = m.gameHistory[len(m.gameHistory)-6:]
				}
			}
			
			m.messages = append(m.messages, "")
		}
		return m, nil

	case llmResponseMsg:
		if m.loading && !m.streaming {
			m.messages = m.messages[:len(m.messages)-1]
			if msg.err != nil {
				errorMsg := fmt.Sprintf("Error: %v", msg.err)
				m.messages = append(m.messages, errorMsg)
				m.gameHistory = append(m.gameHistory, "Error: "+msg.err.Error())
			} else {
				m.messages = append(m.messages, msg.response)
				m.gameHistory = append(m.gameHistory, "Narrator: "+msg.response)
			}
			m.messages = append(m.messages, "")
			m.loading = false
			
			// Keep only last 3 exchanges (6 entries: player + narrator pairs)
			if len(m.gameHistory) > 6 {
				m.gameHistory = m.gameHistory[len(m.gameHistory)-6:]
			}
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
				m.messages = append(m.messages, "")
				m.gameHistory = append(m.gameHistory, "Player: "+userInput)
				m.input = ""
				m.loading = true
				m.animationFrame = 0
				m.messages = append(m.messages, "LOADING_ANIMATION")
				
				// Use streaming by default - change to callLLMAsync for non-streaming
				return m, tea.Batch(startLLMStream(m.client, userInput, m.world, m.gameHistory, m.logger, m.debug), animationTimer())
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
		Foreground(lipgloss.Color("6"))

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

func main() {
	// Handle command line flags
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "review", "--review":
			runReviewMode()
			return
		case "rate":
			if len(os.Args) < 4 {
				fmt.Println("Usage: go run . rate <id> <rating> [notes]")
				return
			}
			runRatingMode()
			return
		}
	}
	
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
	}
}

func runReviewMode() {
	logger, err := NewCompletionLogger()
	if err != nil {
		fmt.Printf("Failed to open completion database: %v\n", err)
		return
	}
	defer logger.Close()
	
	completions, err := logger.GetRecentCompletions(10)
	if err != nil {
		fmt.Printf("Failed to get completions: %v\n", err)
		return
	}
	
	if len(completions) == 0 {
		fmt.Println("No completions found. Play the game first to generate data!")
		return
	}
	
	fmt.Printf("Recent completions (%d):\n\n", len(completions))
	
	for _, comp := range completions {
		var metadata CompletionMetadata
		if err := json.Unmarshal([]byte(comp.Metadata), &metadata); err == nil {
			fmt.Printf("[%d] %s | %v | %s\n", 
				comp.ID, 
				comp.Timestamp.Format("15:04:05"),
				metadata.ResponseTime,
				comp.UserInput)
		} else {
			fmt.Printf("[%d] %s | %s\n", comp.ID, comp.Timestamp.Format("15:04:05"), comp.UserInput)
		}
		
		fmt.Printf("Response: %s\n", comp.Response)
		if comp.Rating != nil {
			fmt.Printf("Rating: %d/5", *comp.Rating)
			if comp.Notes != nil {
				fmt.Printf(" - %s", *comp.Notes)
			}
		} else {
			fmt.Printf("Rating: not rated")
		}
		fmt.Println("\n" + strings.Repeat("-", 50))
	}
	
	fmt.Println("\nTo rate a completion: go run . rate <id> <rating> [notes]")
}

func runRatingMode() {
	id, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Printf("Invalid ID: %v\n", err)
		return
	}
	
	rating, err := strconv.Atoi(os.Args[3])
	if err != nil {
		fmt.Printf("Invalid rating: %v\n", err)
		return
	}
	
	if rating < 1 || rating > 5 {
		fmt.Println("Rating must be between 1 and 5")
		return
	}
	
	var notes string
	if len(os.Args) > 4 {
		notes = strings.Join(os.Args[4:], " ")
	}
	
	logger, err := NewCompletionLogger()
	if err != nil {
		fmt.Printf("Failed to open completion database: %v\n", err)
		return
	}
	defer logger.Close()
	
	err = logger.RateCompletion(id, rating, notes)
	if err != nil {
		fmt.Printf("Failed to rate completion: %v\n", err)
		return
	}
	
	fmt.Printf("Rated completion %d as %d/5", id, rating)
	if notes != "" {
		fmt.Printf(" with notes: %s", notes)
	}
	fmt.Println()
}