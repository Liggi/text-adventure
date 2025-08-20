package ui

import (
	"fmt"
	"time"
	
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sashabaranov/go-openai"
	
	"textadventure/internal/game"
	"textadventure/internal/game/sensory"
	"textadventure/internal/logging"
	"textadventure/internal/mcp"
)


type TurnPhase int

const (
	PlayerTurn TurnPhase = iota
	NPCTurns
	Narration
)

type Model struct {
	messages                []string
	input                   string
	cursor                  int
	width                   int
	height                  int
	client                  *openai.Client
	mcpClient               *mcp.WorldStateClient
	debug                   bool
	loading                 bool
	streaming               bool
	currentResponse         string
	animationFrame          int
	world                   game.WorldState
	gameHistory             []string
	logger                  *logging.CompletionLogger
	turnPhase               TurnPhase
	npcTurnComplete         bool
	accumulatedSensoryEvents []sensory.SensoryEvent
}

func NewModel(
	client *openai.Client,
	world game.WorldState,
	logger *logging.CompletionLogger,
	debug bool,
) Model {
	messages := []string{}
	if debug {
		messages = append(messages, "[DEBUG] MCP integration active - world state loaded from server")
		messages = append(messages, fmt.Sprintf("[DEBUG] Player location: %s, Inventory: %v", world.Location, world.Inventory))
		messages = append(messages, "[DEBUG] Debug commands: /worldstate, /help")
		messages = append(messages, "")
	}
	
	return Model{
		messages:                messages,
		input:                   "",
		cursor:                  0,
		client:                  client,
		debug:                   debug,
		world:                   world,
		gameHistory:             []string{},
		logger:                  logger,
		turnPhase:               PlayerTurn,
		npcTurnComplete:         false,
		accumulatedSensoryEvents: []sensory.SensoryEvent{},
	}
}

func (m *Model) SetMCPClient(client *mcp.WorldStateClient) {
	m.mcpClient = client
}

func (m Model) Init() tea.Cmd {
	return initialLookAroundCmd()
}

type animationTickMsg struct{}

type initialLookAroundMsg struct{}

type npcTurnMsg struct{
	sensoryEvents *sensory.SensoryEventResponse
}

type narrationTurnMsg struct {
	world       game.WorldState
	gameHistory []string
	debug       bool
}


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
	world         game.WorldState
	userInput     string
	systemPrompt  string
	response      string
	startTime     time.Time
	logger        *logging.CompletionLogger
	debug         bool
	sensoryEvents *sensory.SensoryEventResponse
}

type streamStartedMsg struct {
	stream        *openai.ChatCompletionStream
	debug         bool
	world         game.WorldState
	userInput     string
	systemPrompt  string
	startTime     time.Time
	logger        *logging.CompletionLogger
	sensoryEvents *sensory.SensoryEventResponse
}

func initialLookAroundCmd() tea.Cmd {
	return func() tea.Msg {
		return initialLookAroundMsg{}
	}
}

func npcTurnCmd(sensoryEvents *sensory.SensoryEventResponse) tea.Cmd {
	return func() tea.Msg {
		return npcTurnMsg{sensoryEvents: sensoryEvents}
	}
}

func startNarrationCmd(world game.WorldState, gameHistory []string, debug bool) tea.Cmd {
	return func() tea.Msg {
		return narrationTurnMsg{
			world:       world,
			gameHistory: gameHistory,
			debug:       debug,
		}
	}
}