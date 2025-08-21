package ui

import (
	"fmt"
	
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sashabaranov/go-openai"
	
	"textadventure/internal/debug"
	"textadventure/internal/game"
	"textadventure/internal/game/director"
	"textadventure/internal/game/sensory"
	"textadventure/internal/llm"
	"textadventure/internal/logging"
	"textadventure/internal/mcp"
)

type GameLoggers struct {
	Debug      *debug.Logger
	Completion *logging.CompletionLogger
}

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
	llmService              *llm.Service
	mcpClient               *mcp.WorldStateClient
	loggers                 GameLoggers
	director                *director.Director
	loading                 bool
	streaming               bool
	currentResponse         string
	animationFrame          int
	world                   game.WorldState
	gameHistory             *game.History
	logger                  *logging.CompletionLogger
	turnPhase               TurnPhase
	npcTurnComplete         bool
	accumulatedSensoryEvents []sensory.SensoryEvent
}

func NewModel(
	client *openai.Client,
	llmService *llm.Service,
	mcpClient *mcp.WorldStateClient,
	loggers GameLoggers,
	world game.WorldState,
) Model {
	messages := []string{}
	if loggers.Debug.IsEnabled() {
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
		llmService:              llmService,
		mcpClient:               mcpClient,
		loggers:                 loggers,
		director:                director.NewDirector(llmService, mcpClient, loggers.Debug),
		world:                   world,
		gameHistory:             game.NewHistory(6),
		turnPhase:               PlayerTurn,
		npcTurnComplete:         false,
		accumulatedSensoryEvents: []sensory.SensoryEvent{},
	}
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