package ui

import (
    "context"
    "fmt"
    "strings"
    "time"
    
    tea "github.com/charmbracelet/bubbletea"
    "github.com/google/uuid"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
    
    "textadventure/internal/debug"
    "textadventure/internal/game"
    "textadventure/internal/game/director"
    "textadventure/internal/game/facts"
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

func (m *Model) extractAndAccumulateFacts(narrationText string) {
    if strings.TrimSpace(narrationText) == "" {
        return
    }
    
    currentLocation := m.world.Locations[m.world.Location]
    ctx := m.createGameContext(m.sessionContext, "facts.extract")
    
    extractedFacts, err := facts.ExtractLocationFacts(ctx, m.llmService, narrationText, m.world.Location, currentLocation.Facts)
    if err != nil {
        if m.loggers.Debug.IsEnabled() {
            m.loggers.Debug.Errorf("Fact extraction failed: %v", err)
            m.messages = append(m.messages, "\033[31m[ERROR] Fact extraction failed\033[0m")
        }
        return
    }
    
    if len(extractedFacts) > 0 {
        if m.loggers.Debug.IsEnabled() {
            header := "[DEBUG] Facts extracted:"
            m.loggers.Debug.Printf(header)
            m.messages = append(m.messages, header)
            for _, f := range extractedFacts {
                line := "  - " + strings.TrimSpace(f)
                m.loggers.Debug.Printf(line)
                m.messages = append(m.messages, line)
            }
        }
        
        attribution, err := facts.AttributeFacts(ctx, m.llmService, extractedFacts, &m.world)
        if err != nil {
            if m.loggers.Debug.IsEnabled() {
                m.loggers.Debug.Errorf("Fact attribution failed: %v", err)
                m.messages = append(m.messages, "\033[31m[ERROR] Fact attribution failed\033[0m")
            }
            m.world.AccumulateLocationFacts(m.world.Location, extractedFacts)
            return
        }
        
        m.persistAttributedFacts(attribution)
        
        if m.loggers.Debug.IsEnabled() {
            // Show attribution results
            for locationID, facts := range attribution.LocationFacts {
                debugMsg := fmt.Sprintf("[DEBUG] Location %s: %v", locationID, facts)
                m.loggers.Debug.Printf(debugMsg)
                m.messages = append(m.messages, debugMsg)
            }
            for itemID, facts := range attribution.ItemFacts {
                debugMsg := fmt.Sprintf("[DEBUG] Item %s: %v", itemID, facts)
                m.loggers.Debug.Printf(debugMsg)
                m.messages = append(m.messages, debugMsg)
            }
            for npcID, facts := range attribution.NPCFacts {
                debugMsg := fmt.Sprintf("[DEBUG] NPC %s: %v", npcID, facts)
                m.loggers.Debug.Printf(debugMsg)
                m.messages = append(m.messages, debugMsg)
            }
            if len(attribution.Skipped) > 0 {
                debugMsg := fmt.Sprintf("[DEBUG] Skipped: %v", attribution.Skipped)
                m.loggers.Debug.Printf(debugMsg)
                m.messages = append(m.messages, debugMsg)
            }
        }
    } else if m.loggers.Debug.IsEnabled() {
        debugMsg := "[DEBUG] Facts extracted: []"
        m.loggers.Debug.Printf(debugMsg)
        m.messages = append(m.messages, debugMsg)
    }
}

// extractAndAccumulateFactsForLocation runs fact extraction/attribution for a specific location
// (used to attribute NPC-perspective narration to the NPC's current room).
func (m *Model) extractAndAccumulateFactsForLocation(locationID string, narrationText string) {
    if strings.TrimSpace(narrationText) == "" {
        return
    }
    loc, exists := m.world.Locations[locationID]
    if !exists {
        return
    }
    ctx := m.createGameContext(m.sessionContext, "facts.extract")
    extractedFacts, err := facts.ExtractLocationFacts(ctx, m.llmService, narrationText, locationID, loc.Facts)
    if err != nil {
        if m.loggers.Debug.IsEnabled() {
            m.loggers.Debug.Errorf("Fact extraction failed (%s): %v", locationID, err)
            m.messages = append(m.messages, fmt.Sprintf("\033[31m[ERROR] Fact extraction failed for %s\033[0m", locationID))
        }
        return
    }
    if len(extractedFacts) == 0 {
        if m.loggers.Debug.IsEnabled() {
            header := fmt.Sprintf("[DEBUG] Facts extracted for %s:", locationID)
            m.loggers.Debug.Printf(header)
            m.messages = append(m.messages, header)
            m.loggers.Debug.Printf("  - (none)")
            m.messages = append(m.messages, "  - (none)")
        }
        return
    }
    if m.loggers.Debug.IsEnabled() {
        header := fmt.Sprintf("[DEBUG] Facts extracted for %s:", locationID)
        m.loggers.Debug.Printf(header)
        m.messages = append(m.messages, header)
        for _, f := range extractedFacts {
            line := "  - " + strings.TrimSpace(f)
            m.loggers.Debug.Printf(line)
            m.messages = append(m.messages, line)
        }
    }
    attribution, err := facts.AttributeFacts(ctx, m.llmService, extractedFacts, &m.world)
    if err != nil {
        if m.loggers.Debug.IsEnabled() {
            m.loggers.Debug.Errorf("Fact attribution failed (%s): %v", locationID, err)
            m.messages = append(m.messages, fmt.Sprintf("\033[31m[ERROR] Fact attribution failed for %s\033[0m", locationID))
        }
        m.world.AccumulateLocationFacts(locationID, extractedFacts)
        return
    }
    m.persistAttributedFactsForLocation(attribution, locationID)
    if m.loggers.Debug.IsEnabled() {
        for lID, f := range attribution.LocationFacts {
            debugMsg := fmt.Sprintf("[DEBUG] Location %s: %v", lID, f)
            m.loggers.Debug.Printf(debugMsg)
            m.messages = append(m.messages, debugMsg)
        }
        for itemID, f := range attribution.ItemFacts {
            debugMsg := fmt.Sprintf("[DEBUG] Item %s: %v", itemID, f)
            m.loggers.Debug.Printf(debugMsg)
            m.messages = append(m.messages, debugMsg)
        }
        for npcID, f := range attribution.NPCFacts {
            debugMsg := fmt.Sprintf("[DEBUG] NPC %s: %v", npcID, f)
            m.loggers.Debug.Printf(debugMsg)
            m.messages = append(m.messages, debugMsg)
        }
        if len(attribution.Skipped) > 0 {
            debugMsg := fmt.Sprintf("[DEBUG] Skipped: %v", attribution.Skipped)
            m.loggers.Debug.Printf(debugMsg)
            m.messages = append(m.messages, debugMsg)
        }
    }
}

func (m *Model) persistAttributedFacts(attribution *facts.FactAttribution) {
    m.persistAttributedFactsForLocation(attribution, m.world.Location)
}

// persistAttributedFactsForLocation persists attributed facts, scoping item creation to the observer's location.
func (m *Model) persistAttributedFactsForLocation(attribution *facts.FactAttribution, observerLocationID string) {
    ctx := m.createGameContext(m.sessionContext, "facts.persist")
    
    // Persist location facts
    for locationID, locationFacts := range attribution.LocationFacts {
        if len(locationFacts) > 0 {
            result, err := m.mcpClient.CallTool(ctx, "add_location_facts", map[string]interface{}{
                "location_id": locationID,
                "new_facts":   locationFacts,
            })
            if err != nil && m.loggers.Debug.IsEnabled() {
                m.loggers.Debug.Errorf("Failed to persist location facts for %s: %v", locationID, err)
                m.messages = append(m.messages, fmt.Sprintf("\033[31m[ERROR] Persist location facts failed for %s\033[0m", locationID))
            } else if m.loggers.Debug.IsEnabled() {
                m.loggers.Debug.Printf("Persisted location facts for %s: %s", locationID, result)
            }
            
            // Update local world state
            if loc, exists := m.world.Locations[locationID]; exists {
                m.world.Locations[locationID] = game.LocationInfo{
                    Name:  loc.Name,
                    Facts: append(loc.Facts, locationFacts...),
                    Exits: loc.Exits,
                }
            }
        }
    }
    
    // Create items and persist item facts (assigning to observer's current location)
    for itemID, itemFacts := range attribution.ItemFacts {
        if len(itemFacts) > 0 {
            result, err := m.mcpClient.CallTool(ctx, "create_item", map[string]interface{}{
                "item_id":       itemID,
                "name":          itemID, // Use item_id as name for now
                "location":      observerLocationID,
                "initial_facts": itemFacts,
            })
            if err != nil && m.loggers.Debug.IsEnabled() {
                // Item might already exist, try adding facts instead
                result, err = m.mcpClient.CallTool(ctx, "add_item_facts", map[string]interface{}{
                    "item_id":   itemID,
                    "new_facts": itemFacts,
                })
                if err != nil && m.loggers.Debug.IsEnabled() {
                    m.loggers.Debug.Errorf("Failed to persist item facts for %s: %v", itemID, err)
                    m.messages = append(m.messages, fmt.Sprintf("\033[31m[ERROR] Persist item facts failed for %s\033[0m", itemID))
                } else if m.loggers.Debug.IsEnabled() {
                    m.loggers.Debug.Printf("Added facts to existing item %s: %s", itemID, result)
                }
            } else if m.loggers.Debug.IsEnabled() {
                m.loggers.Debug.Printf("Created item %s: %s", itemID, result)
            }
        }
    }
    
    // Persist NPC facts
    for npcID, npcFacts := range attribution.NPCFacts {
        if len(npcFacts) > 0 {
            result, err := m.mcpClient.CallTool(ctx, "add_npc_facts", map[string]interface{}{
                "npc_id":    npcID,
                "new_facts": npcFacts,
            })
            if err != nil && m.loggers.Debug.IsEnabled() {
                m.loggers.Debug.Errorf("Failed to persist NPC facts for %s: %v", npcID, err)
                m.messages = append(m.messages, fmt.Sprintf("\033[31m[ERROR] Persist NPC facts failed for %s\033[0m", npcID))
            } else if m.loggers.Debug.IsEnabled() {
                m.loggers.Debug.Printf("Persisted NPC facts for %s: %s", npcID, result)
            }
            
            // Update local world state
            if npc, exists := m.world.NPCs[npcID]; exists {
                npc.Facts = append(npc.Facts, npcFacts...)
                m.world.NPCs[npcID] = npc
            }
        }
    }
}
