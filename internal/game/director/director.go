package director

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sashabaranov/go-openai"

	"textadventure/internal/debug"
	"textadventure/internal/game"
	"textadventure/internal/logging"
	"textadventure/internal/mcp"
)

type Director struct {
	client       *openai.Client
	mcpClient    *mcp.WorldStateClient
	debugLogger  *debug.Logger
}

func NewDirector(client *openai.Client, mcpClient *mcp.WorldStateClient, debugLogger *debug.Logger) *Director {
	return &Director{
		client:      client,
		mcpClient:   mcpClient,
		debugLogger: debugLogger,
	}
}

type IntentBuilder struct {
	director    *Director
	intent      string
	world       *game.WorldState
	history     []string
	actorID     string
	logger      *logging.CompletionLogger
}

func (d *Director) ProcessIntent(intent string) *IntentBuilder {
	return &IntentBuilder{
		director: d,
		intent:   intent,
	}
}

func (b *IntentBuilder) WithWorld(world game.WorldState) *IntentBuilder {
	b.world = &world
	return b
}

func (b *IntentBuilder) WithHistory(history []string) *IntentBuilder {
	b.history = history
	return b
}

func (b *IntentBuilder) WithActor(actorID string) *IntentBuilder {
	b.actorID = actorID
	return b
}

func (b *IntentBuilder) WithLogger(logger *logging.CompletionLogger) *IntentBuilder {
	b.logger = logger
	return b
}

func (b *IntentBuilder) Execute() tea.Cmd {
	if b.world == nil {
		panic("world state required - call WithWorld() before Execute()")
	}
	
	return ProcessPlayerAction(
		b.director.client,
		b.intent,
		*b.world,
		b.history,
		b.logger,
		b.director.mcpClient,
		b.director.debugLogger,
		b.actorID,
	)
}