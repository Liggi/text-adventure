package director

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sashabaranov/go-openai"

	"textadventure/internal/debug"
	"textadventure/internal/game"
	"textadventure/internal/game/sensory"
	"textadventure/internal/logging"
	"textadventure/internal/mcp"
)

// Director orchestrates LLM-driven world state mutations in the text adventure game.
// It serves as the central controller that interprets user intent and executes corresponding
// world changes through MCP tools.
type Director struct {
	client       *openai.Client
	mcpClient    *mcp.WorldStateClient
	debugLogger  *debug.Logger
}

// NewDirector creates a new Director with the required dependencies for LLM interaction,
// world state management, and debug logging.
func NewDirector(client *openai.Client, mcpClient *mcp.WorldStateClient, debugLogger *debug.Logger) *Director {
	return &Director{
		client:      client,
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

// Execute processes the configured intent and returns a Bubble Tea command.
// Panics if WithWorld() was not called.
func (b *IntentBuilder) Execute() tea.Cmd {
	if b.world == nil {
		panic("world state required - call WithWorld() before Execute()")
	}
	
	return b.director.ProcessPlayerAction(
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
	NewWorld      game.WorldState
	UserInput     string
	Debug         bool
	ActingNPCID   string
}

// InterpretIntent uses the LLM to understand user input and generate an action plan.
// It analyzes the user's intent in the context of the current world state and returns
// a plan containing the specific MCP tool mutations needed to fulfill that intent.
func (d *Director) InterpretIntent(userInput string, world game.WorldState, gameHistory []string, actingNPCID string) (*ActionPlan, error) {
	ctx := context.Background()
	
	toolDescriptions, err := d.mcpClient.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool descriptions from MCP server: %w", err)
	}

	actionLabel := getActionLabel(actingNPCID)
	systemPrompt := buildSystemPrompt(toolDescriptions, world, gameHistory, actionLabel, actingNPCID)

	req := buildCompletionRequest(systemPrompt, actionLabel, userInput)

	d.debugLogger.Printf("Processing action: %q", userInput)

	resp, err := d.client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("mutation generation failed: %w", err)
	}

	var actionPlan ActionPlan
	content := resp.Choices[0].Message.Content
	
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
	actionPlan, err := d.InterpretIntent(userInput, world, gameHistory, actingNPCID)
	if err != nil {
		return &ExecutionResult{}, fmt.Errorf("failed to generate mutations: %w", err)
	}
	
	if len(actionPlan.Mutations) == 0 {
		return &ExecutionResult{Successes: []string{}, Failures: []string{}}, nil
	}
	
	return d.executeWithRetry(ctx, userInput, world, gameHistory, actingNPCID, actionPlan.Mutations)
}

// ProcessPlayerAction is the main entry point for processing user actions.
// It handles the complete flow from intent interpretation through world state updates
// and sensory event generation, returning a Bubble Tea message.
func (d *Director) ProcessPlayerAction(userInput string, world game.WorldState, gameHistory []string, logger *logging.CompletionLogger, actingNPCID ...string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		
		var npcID string
		if len(actingNPCID) > 0 {
			npcID = actingNPCID[0]
		}
		
		executionResult, err := d.ExecuteIntent(ctx, userInput, world, gameHistory, npcID)
		if err != nil {
			executionResult = &ExecutionResult{
				Successes: []string{},
				Failures:  []string{fmt.Sprintf("Failed to process action: %v", err)},
			}
		}
		
		mcpWorld, err := d.mcpClient.GetWorldState(ctx)
		var newWorld game.WorldState
		if err != nil {
			newWorld = world
		} else {
			newWorld = mcp.MCPToGameWorldState(mcpWorld)
		}
		
		sensoryEvents, err := sensory.GenerateSensoryEvents(d.client, userInput, executionResult.Successes, newWorld, d.debugLogger, npcID)
		if err != nil {
			sensoryEvents = &sensory.SensoryEventResponse{AuditoryEvents: []sensory.SensoryEvent{}}
		}
		
		var allMessages []string
		if d.debugLogger != nil {
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
		
		return MutationsGeneratedMsg{
			Mutations:     allMessages,
			Successes:     executionResult.Successes,
			Failures:      executionResult.Failures,
			SensoryEvents: sensoryEvents,
			NewWorld:      newWorld,
			UserInput:     userInput,
			Debug:         d.debugLogger != nil,
			ActingNPCID:   npcID,
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
			
			retryResp, err := d.InterpretIntent(retryPrompt, world, gameHistory, actingNPCID)
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

// buildSystemPrompt constructs the system prompt for the LLM Director.
// This prompt defines the LLM's role and provides context for decision making.
func buildSystemPrompt(toolDescriptions string, world game.WorldState, gameHistory []string, actionLabel string, actingNPCID string) string {
	return fmt.Sprintf(`You are the Director of a text adventure game. Your role is to understand player intent and generate the specific world mutations needed to make it happen.

%s

WORLD STATE CONTEXT:
%s

RULES:
- Parse the %s and decide what world mutations are needed
- Generate JSON array of mutations using the available tools
- Be conservative - only generate mutations that directly relate to the stated action
- For movement: use move_player tool
- For picking up items: use transfer_item to move from location to player, then add_to_inventory
- For dropping items: use remove_from_inventory, then transfer_item to move to current location
- For examining/looking: usually no mutations needed
- NPCs can only affect items at their current location or their own movement

Return JSON format:
{
  "mutations": [
    {"tool": "move_player", "args": {"location": "kitchen"}},
    {"tool": "transfer_item", "args": {"item": "key", "from_location": "foyer", "to_location": "player"}}
  ]
}

If no mutations needed, return empty mutations array.`, toolDescriptions, game.BuildWorldContext(world, gameHistory, actingNPCID), actionLabel)
}

// buildCompletionRequest creates an OpenAI chat completion request with appropriate settings.
func buildCompletionRequest(systemPrompt, actionLabel, userInput string) openai.ChatCompletionRequest {
	return openai.ChatCompletionRequest{
		Model: "gpt-5-2025-08-07",
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: fmt.Sprintf("%s: %s", actionLabel, userInput),
			},
		},
		MaxCompletionTokens: 400,
		ReasoningEffort:     "minimal",
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	}
}