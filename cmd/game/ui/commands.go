package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sashabaranov/go-openai"
	
	"textadventure/internal/game"
	"textadventure/internal/game/actors"
	"textadventure/internal/game/director"
	"textadventure/internal/game/sensory"
	"textadventure/internal/logging"
	"textadventure/internal/mcp"
)

func animationTimer() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return animationTickMsg{}
	})
}

func startLLMStream(client *openai.Client, userInput string, world game.WorldState, gameHistory []string, logger *logging.CompletionLogger, debug bool, mutationResults []string, sensoryEvents *sensory.SensoryEventResponse, actingNPCID ...string) tea.Cmd {
	return func() tea.Msg {
		if debug {
			log.Printf("Starting LLM stream with input: %q", userInput)
		}
		
		startTime := time.Now()
		worldContext := buildWorldContext(world, gameHistory)
		var mutationContext string
		if len(mutationResults) > 0 {
			mutationContext = "\n\nMUTATIONS THAT JUST OCCURRED:\n" + strings.Join(mutationResults, "\n") + "\n\nThe world state above reflects these changes. Narrate based on what actually happened."
		}

		var sensoryContext string
		if sensoryEvents != nil && len(sensoryEvents.AuditoryEvents) > 0 {
			sensoryContext = "\n\nSENSORY EVENTS THAT OCCURRED:\n"
			for _, event := range sensoryEvents.AuditoryEvents {
				sensoryContext += fmt.Sprintf("- %s (%s volume) at %s\n", event.Description, event.Volume, event.Location)
			}
			sensoryContext += "\nThese are the ONLY sounds/events that occurred. Do not invent additional sounds or sensory details."
		}

		systemPrompt := fmt.Sprintf(`You are the narrator for a text adventure game. You have complete knowledge of the world state.

Your job: Respond to player actions with 2-4 sentence vivid narration that feels natural and immersive.

Rules:
- Stay consistent with the provided world state
- Base your narration on what actually happened (see mutation results)
- If sensory events occurred, incorporate them into your narration
- DO NOT invent new sounds, smells, or sensory events beyond what's listed
- If action succeeded, describe the successful action vividly
- If action failed, explain why and suggest alternatives
- Keep responses concise but atmospheric%s%s`, mutationContext, sensoryContext)

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
			stream:        stream,
			debug:         debug,
			world:         world,
			userInput:     userInput,
			systemPrompt:  systemPrompt,
			startTime:     startTime,
			logger:        logger,
			sensoryEvents: sensoryEvents,
		}
	}
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
			metadata := logging.CompletionMetadata{
				Model:         "gpt-5-2025-08-07",
				MaxTokens:     200,
				ResponseTime:  responseTime,
				StreamingUsed: true,
			}
			
			if logErr := completionCtx.logger.LogCompletion(completionCtx.world, completionCtx.userInput, completionCtx.systemPrompt, fullResponse, metadata); logErr != nil && debug {
				log.Printf("Failed to log completion: %v", logErr)
			}
			
			return llmStreamCompleteMsg{
				world:         completionCtx.world,
				userInput:     completionCtx.userInput,
				systemPrompt:  completionCtx.systemPrompt,
				response:      fullResponse,
				startTime:     completionCtx.startTime,
				logger:        completionCtx.logger,
				debug:         debug,
				sensoryEvents: completionCtx.sensoryEvents,
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
		
		return readNextChunk(stream, debug, completionCtx, fullResponse)()
	}
}

func buildWorldContext(world game.WorldState, gameHistory []string, actingNPCID ...string) string {
	var currentLocation string
	
	if len(actingNPCID) > 0 && actingNPCID[0] != "" {
		npc, exists := world.NPCs[actingNPCID[0]]
		if exists {
			currentLocation = npc.Location
		} else {
			currentLocation = world.Location
		}
	} else {
		currentLocation = world.Location
	}
	
	currentLoc := world.Locations[currentLocation]
	context := "WORLD STATE:\n"
	context += "Current Location: " + currentLoc.Title + " (" + currentLocation + ")\n"
	context += currentLoc.Description + "\n"
	
	// Add NPCs present in this location
	var npcsHere []string
	for npcID, npc := range world.NPCs {
		if npc.Location == currentLocation {
			npcsHere = append(npcsHere, npcID)
		}
	}
	if len(npcsHere) > 0 {
		context += "\nPeople here: " + fmt.Sprintf("%v", npcsHere) + "\n"
	}
	context += "\n"
	
	context += "Available Items Here: " + fmt.Sprintf("%v", currentLoc.Items) + "\n"
	context += "Available Exits: " + fmt.Sprintf("%v", currentLoc.Exits) + "\n"
	context += "Player Inventory: " + fmt.Sprintf("%v", world.Inventory) + "\n\n"

	if len(gameHistory) > 0 {
		context += "RECENT CONVERSATION:\n"
		for _, exchange := range gameHistory {
			context += exchange + "\n"
		}
		context += "\n"
	}

	return context
}

type MutationRequest struct {
	Tool string                 `json:"tool"`
	Args map[string]interface{} `json:"args"`
}

type MutationResponse struct {
	Mutations []MutationRequest `json:"mutations"`
}

func generateMutations(client *openai.Client, userInput string, world game.WorldState, gameHistory []string, mcpClient *mcp.WorldStateClient, debug bool, actingNPCID string) (*MutationResponse, error) {
	worldContext := buildWorldContext(world, gameHistory, actingNPCID)
	
	ctx := context.Background()
	
	toolDescriptions, err := mcpClient.ListTools(ctx)
	if err != nil {
		if debug {
			log.Printf("Failed to get tool descriptions, using fallback: %v", err)
		}
		toolDescriptions = "Error: Could not retrieve tool descriptions from MCP server"
	}
	
	var systemPrompt string
	var actionLabel string
	
	if actingNPCID != "" {
		systemPrompt = fmt.Sprintf(`You are a world state mutation engine for a text adventure game. 

Your job: Analyze the NPC's action and return ONLY the specific world mutations needed.

The NPC %s is acting from their current location. Treat this as a valid NPC action, not a player command.

Available MCP tools:
%s

Return JSON only:
{
  "mutations": [{"tool": "tool_name", "args": {"param": "value"}}],
  "reasoning": "Brief explanation of NPC action"
}

If no mutations needed, return empty mutations array.`, actingNPCID, toolDescriptions)
		actionLabel = fmt.Sprintf("NPC %s ACTION", strings.ToUpper(actingNPCID))
	} else {
		systemPrompt = fmt.Sprintf(`You are a world state mutation engine for a text adventure game. 

Your job: Analyze the player's intent and return ONLY the specific world mutations needed.

Available MCP tools:
%s

Return JSON only:
{
  "mutations": [{"tool": "tool_name", "args": {"param": "value"}}],
  "reasoning": "Brief explanation of intent"
}

If no mutations needed, return empty mutations array.`, toolDescriptions)
		actionLabel = "PLAYER ACTION"
	}

	req := openai.ChatCompletionRequest{
		Model: "gpt-5-2025-08-07",
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: worldContext + actionLabel + ": " + userInput,
			},
		},
		MaxCompletionTokens: 200,
		ReasoningEffort:     "minimal",
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	}

	if debug {
		log.Printf("Generating mutations for input: %q", userInput)
	}

	resp, err := client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("mutation generation failed: %w", err)
	}

	var mutationResp MutationResponse
	content := resp.Choices[0].Message.Content
	
	if debug {
		log.Printf("Raw mutation response: %s", content)
	}
	
	if err := json.Unmarshal([]byte(content), &mutationResp); err != nil {
		return nil, fmt.Errorf("failed to parse mutations: %w", err)
	}

	if debug {
		log.Printf("Parsed mutations: %+v", mutationResp)
	}

	return &mutationResp, nil
}


func executeMutations(ctx context.Context, mutations []MutationRequest, mcpClient *mcp.WorldStateClient, debug bool, world game.WorldState, actingNPCID string) ([]string, []string) {
	var successes []string
	var failures []string
	
	for _, mutation := range mutations {
		if debug {
			log.Printf("Executing mutation: %s with args: %v", mutation.Tool, mutation.Args)
		}
		
		var result string
		var err error
		
		switch mutation.Tool {
		case "add_to_inventory":
			if item, ok := mutation.Args["item"].(string); ok {
				if actingNPCID != "" {
					npc, exists := world.NPCs[actingNPCID]
					if exists {
						result, err = mcpClient.TransferItem(ctx, item, npc.Location, actingNPCID)
					} else {
						err = fmt.Errorf("unknown NPC: %s", actingNPCID)
					}
				} else {
					result, err = mcpClient.AddToInventory(ctx, item)
				}
			} else {
				err = fmt.Errorf("invalid item argument")
			}
		case "remove_from_inventory":
			if item, ok := mutation.Args["item"].(string); ok {
				result, err = mcpClient.RemoveFromInventory(ctx, item)
			} else {
				err = fmt.Errorf("invalid item argument")
			}
		case "move_player":
			if location, ok := mutation.Args["location"].(string); ok {
				result, err = mcpClient.MovePlayer(ctx, location)
			} else {
				err = fmt.Errorf("invalid location argument")
			}
		case "transfer_item":
			item, itemOk := mutation.Args["item"].(string)
			fromLoc, fromOk := mutation.Args["from_location"].(string)
			toLoc, toOk := mutation.Args["to_location"].(string)
			if itemOk && fromOk && toOk {
				result, err = mcpClient.TransferItem(ctx, item, fromLoc, toLoc)
			} else {
				err = fmt.Errorf("invalid transfer arguments")
			}
		case "unlock_door":
			location, locOk := mutation.Args["location"].(string)
			direction, dirOk := mutation.Args["direction"].(string)
			keyItem, keyOk := mutation.Args["key_item"].(string)
			if locOk && dirOk && keyOk {
				result, err = mcpClient.UnlockDoor(ctx, location, direction, keyItem)
			} else {
				err = fmt.Errorf("invalid unlock arguments")
			}
		default:
			err = fmt.Errorf("unknown mutation tool: %s", mutation.Tool)
		}
		
		if err != nil {
			failure := fmt.Sprintf("MUTATION FAILED: %s - %v", mutation.Tool, err)
			failures = append(failures, failure)
			if debug {
				log.Printf("Mutation failed: %s - %v", mutation.Tool, err)
			}
		} else {
			success := fmt.Sprintf("MUTATION SUCCESS: %s - %s", mutation.Tool, result)
			successes = append(successes, success)
			if debug {
				log.Printf("Mutation succeeded: %s - %s", mutation.Tool, result)
			}
		}
	}
	
	return successes, failures
}

func generateAndExecuteMutationsWithRetries(ctx context.Context, client *openai.Client, userInput string, world game.WorldState, gameHistory []string, mcpClient *mcp.WorldStateClient, debug bool, actingNPCID string) ([]string, []string, error) {
	const maxRetries = 3
	var allSuccesses []string
	var finalFailures []string
	
	mutationResp, err := generateMutations(client, userInput, world, gameHistory, mcpClient, debug, actingNPCID)
	if err != nil {
		return nil, nil, fmt.Errorf("initial mutation generation failed: %w", err)
	}
	
	retryCount := 0
	pendingMutations := mutationResp.Mutations
	
	for retryCount <= maxRetries && len(pendingMutations) > 0 {
		if debug && retryCount > 0 {
			log.Printf("Retry attempt %d with %d mutations", retryCount, len(pendingMutations))
		}
		
		successes, failures := executeMutations(ctx, pendingMutations, mcpClient, debug, world, actingNPCID)
		
		allSuccesses = append(allSuccesses, successes...)
		
		if len(failures) == 0 {
			break
		}
		
		if retryCount >= maxRetries {
			finalFailures = append(finalFailures, failures...)
			if debug {
				log.Printf("Max retries reached, giving up on %d failed mutations", len(failures))
			}
			break
		}
		
		retryPrompt := fmt.Sprintf("RETRY: Previous mutations failed. User input: %q\n\nFailed mutations and errors:\n%s\n\nPlease generate corrected mutations or skip mutations that are impossible/malformed.", userInput, strings.Join(failures, "\n"))
		
		retryResp, err := generateMutations(client, retryPrompt, world, gameHistory, mcpClient, debug, actingNPCID)
		if err != nil {
			if debug {
				log.Printf("Retry mutation generation failed: %v", err)
			}
			finalFailures = append(finalFailures, failures...)
			break
		}
		
		pendingMutations = retryResp.Mutations
		retryCount++
	}
	
	return allSuccesses, finalFailures, nil
}

func startTwoStepLLMFlow(client *openai.Client, userInput string, world game.WorldState, gameHistory []string, logger *logging.CompletionLogger, mcpClient *mcp.WorldStateClient, debug bool, actingNPCID ...string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		
		if debug {
			log.Printf("Starting two-step flow for input: %q", userInput)
		}
		
		var npcID string
		if len(actingNPCID) > 0 {
			npcID = actingNPCID[0]
		}
		
		successes, failures, err := generateAndExecuteMutationsWithRetries(ctx, client, userInput, world, gameHistory, mcpClient, debug, npcID)
		if err != nil {
			if debug {
				log.Printf("Mutation retry system failed: %v", err)
			}
			return llmResponseMsg{response: "", err: err}
		}
		
		newWorld := world
		if len(successes) > 0 {
			mcpWorld, err := mcpClient.GetWorldState(ctx)
			if err != nil {
				if debug {
					log.Printf("Failed to refresh world state: %v", err)
				}
			} else {
				newWorld = mcp.MCPToGameWorldState(mcpWorld)
				if debug {
					log.Printf("World state refreshed: player at %s, inventory: %v", newWorld.Location, newWorld.Inventory)
				}
			}
		}
		
		var sensoryEvents *sensory.SensoryEventResponse
		if len(actingNPCID) > 0 && actingNPCID[0] != "" {
			sensoryEvents, err = sensory.GenerateSensoryEvents(client, userInput, successes, newWorld, debug, actingNPCID[0])
		} else {
			sensoryEvents, err = sensory.GenerateSensoryEvents(client, userInput, successes, newWorld, debug)
		}
		if err != nil {
			if debug {
				log.Printf("Failed to generate sensory events: %v", err)
			}
			sensoryEvents = &sensory.SensoryEventResponse{AuditoryEvents: []sensory.SensoryEvent{}}
		}
		
		var allMessages []string
		if debug {
			allMessages = append(allMessages, successes...)
			allMessages = append(allMessages, failures...)
		}
		
		return mutationsGeneratedMsg{
			mutations:     allMessages,
			successes:     successes,
			failures:      failures,
			sensoryEvents: sensoryEvents,
			newWorld:      newWorld,
			userInput:     userInput,
			debug:         debug,
			actingNPCID:   npcID,
		}
	}
}

func buildNPCWorldContext(npcID string, world game.WorldState, gameHistory []string) string {
	npc, exists := world.NPCs[npcID]
	if !exists {
		return buildWorldContext(world, gameHistory)
	}
	
	currentLoc := world.Locations[npc.Location]
	context := "WORLD STATE:\n"
	context += "Current Location: " + currentLoc.Title + " (" + npc.Location + ")\n"
	context += currentLoc.Description + "\n"
	
	var peopleHere []string
	if world.Location == npc.Location {
		peopleHere = append(peopleHere, "player")
	}
	for otherNPCID, otherNPC := range world.NPCs {
		if otherNPCID != npcID && otherNPC.Location == npc.Location {
			peopleHere = append(peopleHere, otherNPCID)
		}
	}
	if len(peopleHere) > 0 {
		context += "\nPeople here: " + fmt.Sprintf("%v", peopleHere) + "\n"
	}
	context += "\n"
	
	context += "Available Items Here: " + fmt.Sprintf("%v", currentLoc.Items) + "\n"
	context += "Available Exits: " + fmt.Sprintf("%v", currentLoc.Exits) + "\n"
	
	if world.Location == npc.Location {
		context += "Player Inventory: " + fmt.Sprintf("%v", world.Inventory) + "\n"
	}
	context += "\n"

	if len(gameHistory) > 0 {
		context += "RECENT CONVERSATION:\n"
		for _, exchange := range gameHistory {
			context += exchange + "\n"
		}
		context += "\n"
	}

	return context
}



func buildNPCWorldContextWithSenses(npcID string, world game.WorldState, sensoryEvents *sensory.SensoryEventResponse) string {
	npc, exists := world.NPCs[npcID]
	if !exists {
		return "ERROR: NPC not found"
	}
	
	currentLoc := world.Locations[npc.Location]
	context := "WORLD STATE:\n"
	context += "Current Location: " + currentLoc.Title + " (" + npc.Location + ")\n"
	context += currentLoc.Description + "\n"
	
	var peopleHere []string
	if world.Location == npc.Location {
		peopleHere = append(peopleHere, "player")
	}
	for otherNPCID, otherNPC := range world.NPCs {
		if otherNPCID != npcID && otherNPC.Location == npc.Location {
			peopleHere = append(peopleHere, otherNPCID)
		}
	}
	if len(peopleHere) > 0 {
		context += "\nPeople here: " + fmt.Sprintf("%v", peopleHere) + "\n"
	}
	context += "\n"
	
	context += "Available Items Here: " + fmt.Sprintf("%v", currentLoc.Items) + "\n"
	context += "Available Exits: " + fmt.Sprintf("%v", currentLoc.Exits) + "\n"
	
	if world.Location == npc.Location {
		context += "Player Inventory: " + fmt.Sprintf("%v", world.Inventory) + "\n"
	}
	context += "\n"

	// Add sensory events that Elena can perceive
	if sensoryEvents != nil && len(sensoryEvents.AuditoryEvents) > 0 {
		context += "RECENT SOUNDS:\n"
		for _, event := range sensoryEvents.AuditoryEvents {
			distance := sensory.CalculateRoomDistance(npc.Location, event.Location, world.Locations)
			decayedVolume := sensory.ApplyVolumeDecay(event.Volume, distance)
			
			if decayedVolume != "" {
				if distance == 0 {
					context += fmt.Sprintf("- %s (heard clearly)\n", event.Description)
				} else {
					context += fmt.Sprintf("- %s (heard %s from %s)\n", event.Description, decayedVolume, event.Location)
				}
			}
		}
		context += "\n"
	}

	return context
}

func generateNPCThoughts(client *openai.Client, npcID string, world game.WorldState, gameHistory []string, debug bool, sensoryEvents *sensory.SensoryEventResponse) tea.Cmd {
	return func() tea.Msg {
		if debug {
			worldContext := actors.BuildNPCWorldContextWithSenses(npcID, world, sensoryEvents)
			
			if debug {
				log.Printf("=== NPC BRAIN: %s ===", npcID)
				log.Printf("World context sent to %s:", npcID)
				log.Printf("%s", worldContext)
				log.Printf("=== END NPC CONTEXT ===")
			}
			
			systemPrompt := fmt.Sprintf(`You are %s, an NPC in the game world. Generate realistic internal thoughts based on your current situation.

CONTEXT EXPLANATION:
- WORLD STATE: Describes your current room and environment
- Available Items Here: Items you can see in your room (you don't have them, they're just present)
- People here: Other people in your room with you
- RECENT SOUNDS: Actual sounds you heard (if any). If no sounds listed, everything is quiet
- Only react to information actually provided - don't invent events that didn't happen

Your thoughts should be:
- Brief and natural (10-20 words total)
- Single line, not multiple sentences  
- Based only on what you can actually observe or hear
- Simple language, like actual inner monologue

Examples:
"Quiet in here, wonder what they're doing."
"Heard footsteps from the foyer."
"Still got that journal sitting here."

Return only your thoughts, nothing else. Keep it to one line.`, npcID)

			req := openai.ChatCompletionRequest{
				Model: "gpt-5-2025-08-07",
				Messages: []openai.ChatCompletionMessage{
					{
						Role:    openai.ChatMessageRoleSystem,
						Content: systemPrompt,
					},
					{
						Role:    openai.ChatMessageRoleUser,
						Content: worldContext + "\n\nGenerate your internal thoughts about recent events.",
					},
				},
				MaxCompletionTokens: 100,
				ReasoningEffort:     "minimal",
			}

			resp, err := client.CreateChatCompletion(context.Background(), req)
			if err != nil {
				if debug {
					log.Printf("NPC brain error for %s: %v", npcID, err)
				}
				return actors.NPCThoughtsMsg{
					NPCID:    npcID,
					Thoughts: fmt.Sprintf("*%s seems distracted*", npcID),
					Debug:    debug,
				}
			}

			thoughts := resp.Choices[0].Message.Content
			if debug {
				log.Printf("NPC %s generated thoughts: %s", npcID, thoughts)
			}
			
			return actors.NPCThoughtsMsg{
				NPCID:    npcID,
				Thoughts: thoughts,
				Debug:    debug,
			}
		}
		
		return actors.NPCThoughtsMsg{
			NPCID:    npcID,
			Thoughts: "",
			Debug:    debug,
		}
	}
}

func generateNPCAction(client *openai.Client, npcID string, npcThoughts string, world game.WorldState, sensoryEvents *sensory.SensoryEventResponse, debug bool) (string, error) {
	if npcThoughts == "" {
		return "", nil
	}

	worldContext := actors.BuildNPCWorldContextWithSenses(npcID, world, sensoryEvents)
	
	systemPrompt := `Based on your current thoughts and situation, decide what ONE action you want to take, or do nothing.

CRITICAL: You can only do ONE thing per turn. Choose the most important action based on your thoughts.

Express your action as a simple, clear statement of intent. Your action should follow logically from your current thoughts.

You can take any reasonable action - move somewhere, interact with objects, investigate sounds, communicate, or simply observe. 

Return only a brief action statement, or an empty string if you don't want to act.`

	req := openai.ChatCompletionRequest{
		Model: "gpt-5-2025-08-07",
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: worldContext + "\n\nYour current thoughts: " + npcThoughts + "\n\nWhat action do you want to take?",
			},
		},
		MaxCompletionTokens: 50,
		ReasoningEffort:     "minimal",
	}

	resp, err := client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("failed to generate NPC action: %w", err)
	}

	action := strings.TrimSpace(resp.Choices[0].Message.Content)
	
	if debug {
		log.Printf("NPC %s decided to: \"%s\"", npcID, action)
	}

	return action, nil
}

func generateNPCTurn(client *openai.Client, npcID string, world game.WorldState, gameHistory []string, debug bool, sensoryEvents *sensory.SensoryEventResponse) tea.Cmd {
	return func() tea.Msg {
		thoughts := ""
		if debug {
			worldContext := actors.BuildNPCWorldContextWithSenses(npcID, world, sensoryEvents)
			
			if debug {
				log.Printf("=== NPC BRAIN: %s ===", npcID)
				log.Printf("World context sent to %s:", npcID)
				log.Printf("%s", worldContext)
				log.Printf("=== END NPC CONTEXT ===")
			}
			
			systemPrompt := fmt.Sprintf(`You are %s, an NPC in the game world. Generate realistic internal thoughts based on your current situation.

CONTEXT EXPLANATION:
- WORLD STATE: Describes your current room and environment
- Available Items Here: Items you can see in your room (you don't have them, they're just present)
- People here: Other people in your room with you
- RECENT SOUNDS: Actual sounds you heard (if any). If no sounds listed, everything is quiet
- Only react to information actually provided - don't invent events that didn't happen

Your thoughts should be:
- Brief and natural (10-20 words total)
- Single line, not multiple sentences  
- Based only on what you can actually observe or hear
- Simple language, like actual inner monologue

Return only your thoughts, nothing else. Keep it to one line.`, npcID)

			req := openai.ChatCompletionRequest{
				Model: "gpt-5-2025-08-07",
				Messages: []openai.ChatCompletionMessage{
					{
						Role:    openai.ChatMessageRoleSystem,
						Content: systemPrompt,
					},
					{
						Role:    openai.ChatMessageRoleUser,
						Content: worldContext + "\n\nGenerate your internal thoughts about recent events.",
					},
				},
				MaxCompletionTokens: 100,
				ReasoningEffort:     "minimal",
			}

			resp, err := client.CreateChatCompletion(context.Background(), req)
			if err != nil {
				if debug {
					log.Printf("NPC brain error for %s: %v", npcID, err)
				}
				thoughts = fmt.Sprintf("*%s seems distracted*", npcID)
			} else {
				thoughts = resp.Choices[0].Message.Content
				if debug {
					log.Printf("NPC %s generated thoughts: %s", npcID, thoughts)
				}
			}
		}
		
		action, err := actors.GenerateNPCAction(client, npcID, thoughts, world, sensoryEvents, debug)
		if err != nil && debug {
			log.Printf("NPC action generation error for %s: %v", npcID, err)
			action = ""
		}
		
		return actors.NPCActionMsg{
			NPCID:         npcID,
			Thoughts:      thoughts,
			Action:        action,
			SensoryEvents: sensoryEvents,
			Debug:         debug,
		}
	}
}