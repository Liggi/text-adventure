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

type ActionPlan struct {
	Mutations []MutationRequest `json:"mutations"`
}

type ExecutionResult struct {
	Successes []string
	Failures  []string
}

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

func InterpretIntent(client *openai.Client, userInput string, world game.WorldState, gameHistory []string, mcpClient *mcp.WorldStateClient, debugLogger *debug.Logger, actingNPCID string) (*ActionPlan, error) {
	ctx := context.Background()
	
	toolDescriptions, err := mcpClient.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool descriptions from MCP server: %w", err)
	}

	var actionLabel string
	if actingNPCID != "" {
		actionLabel = fmt.Sprintf("NPC %s ACTION", strings.ToUpper(actingNPCID))
	} else {
		actionLabel = "Player action"
	}

	systemPrompt := fmt.Sprintf(`You are the Director of a text adventure game. Your role is to understand player intent and generate the specific world mutations needed to make it happen.

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

If no mutations needed, return empty mutations array.`, toolDescriptions, buildWorldContext(world, gameHistory, actingNPCID), actionLabel)

	req := openai.ChatCompletionRequest{
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

	debugLogger.Printf("=== MUTATION GENERATION START ===")
	debugLogger.Printf("Action: %q", userInput)
	debugLogger.Printf("System prompt length: %d chars", len(systemPrompt))

	resp, err := client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		debugLogger.Printf("Mutation generation API error: %v", err)
		return nil, fmt.Errorf("mutation generation failed: %w", err)
	}

	debugLogger.Printf("API Response - Choices length: %d", len(resp.Choices))
	if len(resp.Choices) > 0 {
		debugLogger.Printf("Response choice 0 - Content: %q", resp.Choices[0].Message.Content)
	}

	var actionPlan ActionPlan
	content := resp.Choices[0].Message.Content
	
	if err := json.Unmarshal([]byte(content), &actionPlan); err != nil {
		debugLogger.Printf("JSON unmarshal failed: %v", err)
		debugLogger.Printf("Content was: %q", content)
		return &ActionPlan{Mutations: []MutationRequest{}}, nil
	}

	debugLogger.Printf("Generated %d mutations", len(actionPlan.Mutations))
	for i, mutation := range actionPlan.Mutations {
		debugLogger.Printf("  Mutation %d: %s with args %v", i, mutation.Tool, mutation.Args)
	}
	debugLogger.Printf("=== MUTATION GENERATION END ===")

	return &actionPlan, nil
}

func ExecuteIntent(ctx context.Context, client *openai.Client, userInput string, world game.WorldState, gameHistory []string, mcpClient *mcp.WorldStateClient, debugLogger *debug.Logger, actingNPCID string) (*ExecutionResult, error) {
	actionPlan, err := InterpretIntent(client, userInput, world, gameHistory, mcpClient, debugLogger, actingNPCID)
	if err != nil {
		return &ExecutionResult{}, fmt.Errorf("failed to generate mutations: %w", err)
	}
	
	if len(actionPlan.Mutations) == 0 {
		return &ExecutionResult{Successes: []string{}, Failures: []string{}}, nil
	}
	
	pendingMutations := actionPlan.Mutations
	var allSuccesses []string
	var allFailures []string
	
	for attempt := 0; attempt < 2 && len(pendingMutations) > 0; attempt++ {
		debugLogger.Printf("Mutation attempt %d with %d mutations", attempt+1, len(pendingMutations))
		
		successes, failures := ExecuteMutations(ctx, pendingMutations, mcpClient, debugLogger, world, actingNPCID)
		allSuccesses = append(allSuccesses, successes...)
		
		if len(failures) == 0 {
			break
		}
		
		allFailures = append(allFailures, failures...)
		
		if attempt == 0 && len(failures) > 0 {
			retryPrompt := fmt.Sprintf("Previous attempt failed with errors: %s. Please try a different approach for: %s", 
				strings.Join(failures, "; "), userInput)
			
			retryResp, err := InterpretIntent(client, retryPrompt, world, gameHistory, mcpClient, debugLogger, actingNPCID)
			if err != nil {
				debugLogger.Printf("Retry mutation generation failed: %v", err)
				break
			}
			pendingMutations = retryResp.Mutations
		} else {
			break
		}
	}
	
	return &ExecutionResult{Successes: allSuccesses, Failures: allFailures}, nil
}

func ProcessPlayerAction(client *openai.Client, userInput string, world game.WorldState, gameHistory []string, logger *logging.CompletionLogger, mcpClient *mcp.WorldStateClient, debugLogger *debug.Logger, actingNPCID ...string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		
		var npcID string
		if len(actingNPCID) > 0 {
			npcID = actingNPCID[0]
		}
		
		executionResult, err := ExecuteIntent(ctx, client, userInput, world, gameHistory, mcpClient, debugLogger, npcID)
		if err != nil {
			debugLogger.Printf("Mutation generation/execution failed: %v", err)
			executionResult = &ExecutionResult{
				Successes: []string{},
				Failures:  []string{fmt.Sprintf("Failed to process action: %v", err)},
			}
		}
		
		mcpWorld, err := mcpClient.GetWorldState(ctx)
		var newWorld game.WorldState
		if err != nil {
			debugLogger.Printf("Failed to get updated world state: %v", err)
			newWorld = world
		} else {
			newWorld = mcp.MCPToGameWorldState(mcpWorld)
		}
		
		sensoryEvents, err := sensory.GenerateSensoryEvents(client, userInput, executionResult.Successes, newWorld, debugLogger != nil, npcID)
		if err != nil {
			debugLogger.Printf("Failed to generate sensory events: %v", err)
			sensoryEvents = &sensory.SensoryEventResponse{AuditoryEvents: []sensory.SensoryEvent{}}
		}
		
		var allMessages []string
		if debugLogger != nil {
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
			Debug:         debugLogger != nil,
			ActingNPCID:   npcID,
		}
	}
}

func buildWorldContext(world game.WorldState, gameHistory []string, actingNPCID ...string) string {
	var context strings.Builder
	
	context.WriteString("WORLD STATE:\n")
	
	if len(actingNPCID) > 0 && actingNPCID[0] != "" {
		npcID := actingNPCID[0]
		if npc, exists := world.NPCs[npcID]; exists {
			currentLoc := world.Locations[npc.Location]
			context.WriteString(fmt.Sprintf("NPC %s Location: %s (%s)\n", npcID, currentLoc.Title, npc.Location))
			context.WriteString(currentLoc.Description + "\n")
			context.WriteString(fmt.Sprintf("Available Items Here: %v\n", currentLoc.Items))
			context.WriteString(fmt.Sprintf("Available Exits: %v\n", currentLoc.Exits))
			
			if world.Location == npc.Location {
				context.WriteString("Player is also here\n")
				context.WriteString(fmt.Sprintf("Player Inventory: %v\n", world.Inventory))
			}
		}
	} else {
		currentLoc := world.Locations[world.Location]
		context.WriteString("Player Location: " + currentLoc.Title + " (" + world.Location + ")\n")
		context.WriteString(currentLoc.Description + "\n")
		context.WriteString(fmt.Sprintf("Player Inventory: %v\n", world.Inventory))
		context.WriteString(fmt.Sprintf("Available Items Here: %v\n", currentLoc.Items))
		context.WriteString(fmt.Sprintf("Available Exits: %v\n", currentLoc.Exits))
	}
	
	context.WriteString("\n")
	
	if len(gameHistory) > 0 {
		context.WriteString("RECENT CONVERSATION:\n")
		for _, exchange := range gameHistory {
			context.WriteString(exchange + "\n")
		}
		context.WriteString("\n")
	}
	
	return context.String()
}