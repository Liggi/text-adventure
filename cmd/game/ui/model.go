package ui

import (
    "context"
    "fmt"
    "time"
    
    tea "github.com/charmbracelet/bubbletea"
    "github.com/google/uuid"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
    
    "textadventure/internal/debug"
    "textadventure/internal/game"
    "textadventure/internal/game/director"
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

func (tp TurnPhase) String() string {
	switch tp {
	case PlayerTurn:
		return "player_turn"
	case NPCTurns:
		return "npc_turns"
	case Narration:
		return "narration"
	default:
		return "unknown"
	}
}

type Model struct {
	messages                []string
	input                   string
	cursor                  int
	width                   int
	height                  int
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
    accumulatedWorldEvents  []string
    sessionID               string
    sessionStartTime        time.Time
    sessionContext          context.Context
    sessionSpan             trace.Span
    turnID                  string
    turnIndex               int
    turnContext             context.Context
    turnSpan                trace.Span
}

func NewModel(
	llmService *llm.Service,
	mcpClient *mcp.WorldStateClient,
	loggers GameLoggers,
	world game.WorldState,
) Model {
	messages := []string{}
	sessionID := uuid.New().String()
	sessionStartTime := time.Now()
	
	tracer := otel.Tracer("text-adventure-ui")
	sessionCtx, sessionSpan := tracer.Start(context.Background(), "game-session",
		trace.WithAttributes(
			attribute.String("langfuse.session.id", sessionID),
			attribute.String("session.id", sessionID),
			attribute.String("langfuse.trace.name", "text-adventure"),
			attribute.String("langfuse.trace.metadata.session_id", sessionID),
			attribute.String("game.initial_location", world.Location),
			attribute.Int("game.initial_inventory_count", len(world.Inventory)),
			attribute.String("langfuse.trace.tags", "game,session"),
		),
	)
	
	if loggers.Debug.IsEnabled() {
		messages = append(messages, "[DEBUG] MCP integration active - world state loaded from server")
		messages = append(messages, fmt.Sprintf("[DEBUG] Player location: %s, Inventory: %v", world.Location, world.Inventory))
		messages = append(messages, "[DEBUG] Debug commands: /worldstate, /help")
		messages = append(messages, fmt.Sprintf("[DEBUG] Session ID: %s", sessionID[:8]))
		messages = append(messages, "")
	}
	
    return Model{
		messages:                messages,
		input:                   "",
		cursor:                  0,
		llmService:              llmService,
		mcpClient:               mcpClient,
		loggers:                 loggers,
		director:                director.NewDirector(llmService, mcpClient, loggers.Debug),
		world:                   world,
		gameHistory:             game.NewHistory(6),
		turnPhase:               PlayerTurn,
		npcTurnComplete:         false,
        accumulatedWorldEvents:  []string{},
		sessionID:               sessionID,
        sessionStartTime:        sessionStartTime,
        sessionContext:          sessionCtx,
        sessionSpan:             sessionSpan,
        turnID:                  "",
        turnIndex:               0,
        turnContext:             nil,
        turnSpan:                nil,
    }
}


func (m Model) Init() tea.Cmd {
	return initialLookAroundCmd()
}

type animationTickMsg struct{}

type initialLookAroundMsg struct{}

type npcTurnMsg struct{
    worldEventLines []string
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

func npcTurnCmd(worldEventLines []string) tea.Cmd {
    return func() tea.Msg {
        return npcTurnMsg{worldEventLines: worldEventLines}
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

func (m Model) createGameContext(ctx context.Context, operationType string) context.Context {
	sessionDuration := time.Since(m.sessionStartTime)
	
	gameCtx := map[string]interface{}{
		"location":         m.world.Location,
		"inventory_count":  len(m.world.Inventory),
		"turn_phase":       m.turnPhase.String(),
		"session_duration": int(sessionDuration.Minutes()),
	}
	
    if len(m.world.Inventory) > 0 {
        gameCtx["inventory"] = m.world.Inventory
    }
    if m.turnID != "" {
        gameCtx["turn_id"] = m.turnID
        gameCtx["turn_index"] = m.turnIndex
    }
	
	enrichedCtx := llm.WithSessionID(ctx, m.sessionID)
	enrichedCtx = llm.WithOperationType(enrichedCtx, operationType)
	enrichedCtx = llm.WithGameContext(enrichedCtx, gameCtx)
	
	return enrichedCtx
}

func (m Model) Cleanup() {
	if m.sessionSpan != nil {
		sessionDuration := time.Since(m.sessionStartTime)
		m.sessionSpan.SetAttributes(
			attribute.Int64("game.session_duration_seconds", int64(sessionDuration.Seconds())),
			attribute.String("game.session_end_reason", "normal_exit"),
		)
		m.sessionSpan.End()
	}
}

// startTurn initializes a new turn span and context under the session.
func (m *Model) startTurn() {
    // End any dangling turn span first
    if m.turnSpan != nil {
        m.turnSpan.End()
        m.turnSpan = nil
    }
    m.turnIndex++
    m.turnID = uuid.New().String()
    tracer := otel.Tracer("text-adventure-ui")
    ctx, span := tracer.Start(m.sessionContext, "game.turn",
        trace.WithAttributes(
            attribute.String("turn.id", m.turnID),
            attribute.Int("turn.index", m.turnIndex),
            attribute.String("turn.phase", m.turnPhase.String()),
            attribute.String("location", m.world.Location),
            attribute.Int("inventory_count", len(m.world.Inventory)),
        ),
    )
    m.turnContext = ctx
    m.turnSpan = span
}

// endTurn finalizes the current turn span, if any.
func (m *Model) endTurn(endReason string) {
    if m.turnSpan != nil {
        m.turnSpan.SetAttributes(
            attribute.String("game.turn_end_reason", endReason),
        )
        m.turnSpan.End()
        m.turnSpan = nil
        m.turnContext = nil
        m.turnID = ""
    }
}
