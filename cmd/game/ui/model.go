package ui

import (
	"time"
	
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sashabaranov/go-openai"
	
	"textadventure/internal/game"
	"textadventure/internal/logging"
)

type Model struct {
	messages        []string
	input           string
	cursor          int
	width           int
	height          int
	client          *openai.Client
	debug           bool
	loading         bool
	streaming       bool
	currentResponse string
	animationFrame  int
	world           game.WorldState
	gameHistory     []string
	logger          *logging.CompletionLogger
}

func NewModel(
	client *openai.Client,
	world game.WorldState,
	logger *logging.CompletionLogger,
	debug bool,
) Model {
	return Model{
		messages:    []string{},
		input:       "",
		cursor:      0,
		client:      client,
		debug:       debug,
		world:       world,
		gameHistory: []string{},
		logger:      logger,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

type animationTickMsg struct{}

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
	world        game.WorldState
	userInput    string
	systemPrompt string
	response     string
	startTime    time.Time
	logger       *logging.CompletionLogger
	debug        bool
}

type streamStartedMsg struct {
	stream       *openai.ChatCompletionStream
	debug        bool
	world        game.WorldState
	userInput    string
	systemPrompt string
	startTime    time.Time
	logger       *logging.CompletionLogger
}