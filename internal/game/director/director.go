package director

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"

    tea "github.com/charmbracelet/bubbletea"

    "textadventure/internal/debug"
    "textadventure/internal/game"
    "textadventure/internal/game/sensory"
    "textadventure/internal/llm"
    "textadventure/internal/logging"
    "textadventure/internal/mcp"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
)

// Director orchestrates LLM-driven world state mutations in the text adventure game.
// It serves as the central controller that interprets user intent and executes corresponding
// world changes through MCP tools.
type Director struct {
	llmService   *llm.Service
	mcpClient    *mcp.WorldStateClient
	debugLogger  *debug.Logger
}

// NewDirector creates a new Director with the required dependencies for LLM interaction,
// world state management, and debug logging.
func NewDirector(llmService *llm.Service, mcpClient *mcp.WorldStateClient, debugLogger *debug.Logger) *Director {
	return &Director{
		llmService:  llmService,
		mcpClient:   mcpClient,
		debugLogger: debugLogger,
	}
}

// IntentBuilder provides a fluent interface for configuring and executing user intent processing.
// Use ProcessIntent() to create a builder, configure it with With* methods, then call Execute().
type IntentBuilder struct {
	director    *Director
	intent      string
	world       *game.WorldState
	history     []string
	actorID     string
	logger      *logging.CompletionLogger
}

// ProcessIntent creates a new IntentBuilder for the given user intent string.
// This is the entry point for the fluent API pattern.
func (d *Director) ProcessIntent(intent string) *IntentBuilder {
	return &IntentBuilder{
		director: d,
		intent:   intent,
	}
}

// WithWorld sets the current world state context for intent processing.
// This is required - Execute() will panic if not called.
func (b *IntentBuilder) WithWorld(world game.WorldState) *IntentBuilder {
	b.world = &world
	return b
}

// WithHistory sets the recent conversation history to provide context for the LLM.
func (b *IntentBuilder) WithHistory(history []string) *IntentBuilder {
	b.history = history
	return b
}

// WithActor sets the acting entity ID (empty for player, NPC ID for NPC actions).
func (b *IntentBuilder) WithActor(actorID string) *IntentBuilder {
	b.actorID = actorID
	return b
}

// WithLogger sets the completion logger for request/response logging.
func (b *IntentBuilder) WithLogger(logger *logging.CompletionLogger) *IntentBuilder {
	b.logger = logger
	return b
}

func (b *IntentBuilder) Execute() tea.Cmd {
	return b.ExecuteWithContext(context.Background())
}

func (b *IntentBuilder) ExecuteWithContext(ctx context.Context) tea.Cmd {
	if b.world == nil {
		panic("world state required - call WithWorld() before Execute()")
	}
	
	return b.director.ProcessPlayerActionWithContext(
		ctx,
		b.intent,
		*b.world,
		b.history,
		b.logger,
		b.actorID,
	)
}

// ActionPlan represents the LLM's interpretation of user intent as a series of mutations.
type ActionPlan struct {
	Mutations []MutationRequest `json:"mutations"`
}

// ExecutionResult contains the outcome of executing an action plan.
type ExecutionResult struct {
	Successes []string
	Failures  []string
}

// MutationsGeneratedMsg is the Bubble Tea message sent after processing player actions.
type MutationsGeneratedMsg struct {
    Mutations     []string
    Successes     []string
    Failures      []string
    SensoryEvents *sensory.SensoryEventResponse
    WorldEventLines []string
    NewWorld      game.WorldState
    UserInput     string
    Debug         bool
    ActingNPCID   string
    ActionContext string // What the actor did (for narrator context)
}

// InterpretIntent uses the LLM to understand user input and generate an action plan.
// It analyzes the user's intent in the context of the current world state and returns
// a plan containing the specific MCP tool mutations needed to fulfill that intent.
func (d *Director) InterpretIntent(ctx context.Context, userInput string, world game.WorldState, gameHistory []string, actingNPCID string) (*ActionPlan, error) {
    toolDescriptions, err := d.mcpClient.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool descriptions from MCP server: %w", err)
	}

	actionLabel := getActionLabel(actingNPCID)
	
	req := llm.JSONCompletionRequest{
		SystemPrompt: buildDirectorPrompt(toolDescriptions, world, gameHistory, actionLabel, actingNPCID),
		UserPrompt:   fmt.Sprintf("%s: %s", actionLabel, userInput),
		MaxTokens:    400,
	}

    content, err := d.llmService.CompleteJSON(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("mutation generation failed: %w", err)
	}

	var actionPlan ActionPlan
	
	if err := json.Unmarshal([]byte(content), &actionPlan); err != nil {
		d.debugLogger.Printf("Failed to parse LLM response: %v", err)
		return &ActionPlan{Mutations: []MutationRequest{}}, nil
	}

	if len(actionPlan.Mutations) > 0 {
		d.debugLogger.Printf("Generated %d mutations", len(actionPlan.Mutations))
	}

	return &actionPlan, nil
}

// ExecuteIntent interprets user input and executes the resulting action plan with retry logic.
// It combines intent interpretation with mutation execution, handling failures gracefully.
func (d *Director) ExecuteIntent(ctx context.Context, userInput string, world game.WorldState, gameHistory []string, actingNPCID string) (*ExecutionResult, error) {
    actionPlan, err := d.InterpretIntent(ctx, userInput, world, gameHistory, actingNPCID)
	if err != nil {
		return &ExecutionResult{}, fmt.Errorf("failed to generate mutations: %w", err)
	}
	
	if len(actionPlan.Mutations) == 0 {
		return &ExecutionResult{Successes: []string{}, Failures: []string{}}, nil
	}
	
	return d.executeWithRetry(ctx, userInput, world, gameHistory, actingNPCID, actionPlan.Mutations)
}

func (d *Director) ProcessPlayerAction(userInput string, world game.WorldState, gameHistory []string, logger *logging.CompletionLogger, actingNPCID ...string) tea.Cmd {
	ctx := context.Background()
	return d.ProcessPlayerActionWithContext(ctx, userInput, world, gameHistory, logger, actingNPCID...)
}

func (d *Director) ProcessPlayerActionWithContext(ctx context.Context, userInput string, world game.WorldState, gameHistory []string, logger *logging.CompletionLogger, actingNPCID ...string) tea.Cmd {
    return func() tea.Msg {
        tracer := otel.Tracer("director")
        ctx, span := tracer.Start(ctx, "director.handle_action",
            trace.WithAttributes(
                attribute.String("user.input", userInput),
            ),
        )
        // Attach session/turn/game context to the wrapper span
        llm.CopyGameContextToSpan(ctx, span)
        defer span.End()
        var npcID string
        if len(actingNPCID) > 0 {
            npcID = actingNPCID[0]
        }
        if npcID != "" {
            span.SetAttributes(attribute.String("acting_npc", npcID))
        }
        executionResult, err := d.ExecuteIntent(ctx, userInput, world, gameHistory, npcID)
        if err != nil {
            executionResult = &ExecutionResult{
                Successes: []string{},
                Failures:  []string{fmt.Sprintf("Failed to process action: %v", err)},
            }
            span.RecordError(err)
        }
        
        mcpWorld, err := d.mcpClient.GetWorldState(ctx)
        var newWorld game.WorldState
        if err != nil {
            newWorld = world
        } else {
            newWorld = mcp.MCPToGameWorldState(mcpWorld)
        }

        // Summarize canonical world event lines for this turn using the LLM
        worldEventLines := d.summarizeTurnEvents(ctx, userInput, npcID, world, newWorld, executionResult.Successes, executionResult.Failures)

        var allMessages []string
		if d.debugLogger != nil && d.debugLogger.IsEnabled() {
			allMessages = append(allMessages, "[MUTATIONS]")
			if len(executionResult.Successes) > 0 {
				allMessages = append(allMessages, executionResult.Successes...)
			}
			if len(executionResult.Failures) > 0 {
				for _, failure := range executionResult.Failures {
					allMessages = append(allMessages, "[ERROR] "+failure)
				}
			}
			if len(executionResult.Successes) == 0 && len(executionResult.Failures) == 0 {
				allMessages = append(allMessages, "No mutations needed")
			}
        }

        // Create action context for narrator (what actually happened)
        var actionContext string
        if npcID != "" {
            actionContext = fmt.Sprintf("%s: %s", strings.ToUpper(npcID), userInput)
        } else {
            actionContext = fmt.Sprintf("PLAYER: %s", userInput)
        }

        span.SetAttributes(
            attribute.Int("result.success_count", len(executionResult.Successes)),
            attribute.Int("result.failure_count", len(executionResult.Failures)),
        )

        return MutationsGeneratedMsg{
            Mutations:     allMessages,
            Successes:     executionResult.Successes,
            Failures:      executionResult.Failures,
            SensoryEvents: nil,
            WorldEventLines: worldEventLines,
            NewWorld:      newWorld,
            UserInput:     userInput,
            Debug:         d.debugLogger.IsEnabled(),
            ActingNPCID:   npcID,
            ActionContext: actionContext,
        }
    }
}

// executeWithRetry handles mutation execution with automatic retry on failures.
// If the first attempt fails, it asks the LLM to generate an alternative approach.
func (d *Director) executeWithRetry(ctx context.Context, userInput string, world game.WorldState, gameHistory []string, actingNPCID string, mutations []MutationRequest) (*ExecutionResult, error) {
	pendingMutations := mutations
	var allSuccesses []string
	var allFailures []string
	
	for attempt := 0; attempt < 2 && len(pendingMutations) > 0; attempt++ {
		successes, failures := ExecuteMutations(ctx, pendingMutations, d.mcpClient, d.debugLogger, world, actingNPCID)
		allSuccesses = append(allSuccesses, successes...)
		
		if len(failures) == 0 {
			break
		}
		
		allFailures = append(allFailures, failures...)
		
		if attempt == 0 && len(failures) > 0 {
			retryPrompt := fmt.Sprintf("Previous attempt failed with errors: %s. Please try a different approach for: %s", 
				strings.Join(failures, "; "), userInput)
			
            retryResp, err := d.InterpretIntent(ctx, retryPrompt, world, gameHistory, actingNPCID)
			if err != nil {
				break
			}
			pendingMutations = retryResp.Mutations
		} else {
			break
		}
	}
	
	return &ExecutionResult{Successes: allSuccesses, Failures: allFailures}, nil
}


// getActionLabel returns the appropriate action label for logging and prompts.
func getActionLabel(actingNPCID string) string {
    if actingNPCID != "" {
        return fmt.Sprintf("NPC %s ACTION", strings.ToUpper(actingNPCID))
    }
    return "Player action"
}

// summarizeTurnEvents asks the LLM to produce short, human-readable event lines
// that describe what happened this turn, including successes, non-mutating actions,
// and failures. No invented events.
func (d *Director) summarizeTurnEvents(ctx context.Context, userInput, npcID string, oldWorld, newWorld game.WorldState, successes, failures []string) []string {
    tracer := otel.Tracer("events")
    ctx, span := tracer.Start(ctx, "events.summarize")
    defer span.End()

    actor := "PLAYER"
    if npcID != "" {
        actor = strings.ToUpper(npcID)
    }

    worldDeltaHint := ""
    if oldWorld.Location != newWorld.Location {
        worldDeltaHint = fmt.Sprintf("Location changed: %s -> %s", oldWorld.Location, newWorld.Location)
    }

    sb := &strings.Builder{}
    fmt.Fprintf(sb, "ACTOR: %s\nINPUT: %s\n", actor, userInput)
    if len(successes) > 0 {
        fmt.Fprintf(sb, "SUCCESSES:\n%s\n", strings.Join(successes, "\n"))
    }
    if len(failures) > 0 {
        fmt.Fprintf(sb, "FAILURES:\n%s\n", strings.Join(failures, "\n"))
    }
    if worldDeltaHint != "" {
        fmt.Fprintf(sb, "WORLD HINT: %s\n", worldDeltaHint)
    }

    req := llm.JSONCompletionRequest{
        SystemPrompt: `You summarize the outcome of a single game turn.
Return a JSON array of short, human-readable lines describing what actually happened this turn.
Use present tense. Do not invent events. It's OK if some lines describe attempts that didn't change state (like examining).` ,
        UserPrompt:   sb.String(),
        MaxTokens:    120,
    }

    // Tag this as an event summarization op
    ctx = llm.WithOperationType(ctx, "events.summarize")
    content, err := d.llmService.CompleteJSON(ctx, req)
    if err != nil {
        if d.debugLogger != nil {
            d.debugLogger.Printf("event summarization failed: %v", err)
        }
        // Fallback: derive lines from successes/failures/user input conservatively
        lines := []string{}
        if userInput != "" {
            if npcID != "" {
                lines = append(lines, fmt.Sprintf("%s: %s", actor, userInput))
            } else {
                lines = append(lines, fmt.Sprintf("Player: %s", userInput))
            }
        }
        for _, s := range successes {
            lines = append(lines, s)
        }
        for _, f := range failures {
            lines = append(lines, f)
        }
        return lines
    }
    var arr []string
    if jerr := json.Unmarshal([]byte(content), &arr); jerr != nil {
        if d.debugLogger != nil {
            d.debugLogger.Printf("event summarization JSON parse failed: %v", jerr)
        }
        return []string{}
    }
    return arr
}
