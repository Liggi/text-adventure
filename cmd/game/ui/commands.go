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
	"textadventure/internal/logging"
	"textadventure/internal/mcp"
)

func animationTimer() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return animationTickMsg{}
	})
}

func startLLMStream(client *openai.Client, userInput string, world game.WorldState, gameHistory []string, logger *logging.CompletionLogger, debug bool, mutationResults []string) tea.Cmd {
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

		systemPrompt := fmt.Sprintf(`You are both narrator and world simulator for a text adventure game. You have complete knowledge of the world state.

Your job: Respond to player actions with 2-4 sentence vivid narration that feels natural and immersive.

Rules:
- Stay consistent with the provided world state
- Base your narration on what actually happened (see mutation results)
- If action succeeded, describe the successful action vividly
- If action failed, explain why and suggest alternatives
- Keep responses concise but atmospheric%s`, mutationContext)

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
			stream:       stream,
			debug:        debug,
			world:        world,
			userInput:    userInput,
			systemPrompt: systemPrompt,
			startTime:    startTime,
			logger:       logger,
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
				world:        completionCtx.world,
				userInput:    completionCtx.userInput,
				systemPrompt: completionCtx.systemPrompt,
				response:     fullResponse,
				startTime:    completionCtx.startTime,
				logger:       completionCtx.logger,
				debug:        debug,
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

func buildWorldContext(world game.WorldState, gameHistory []string) string {
	currentLoc := world.Locations[world.Location]
	context := "WORLD STATE:\n"
	context += "Current Location: " + currentLoc.Title + " (" + world.Location + ")\n"
	context += currentLoc.Description + "\n\n"
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
	Reasoning string            `json:"reasoning"`
}

func generateMutations(client *openai.Client, userInput string, world game.WorldState, gameHistory []string, mcpClient *mcp.WorldStateClient, debug bool) (*MutationResponse, error) {
	worldContext := buildWorldContext(world, gameHistory)
	
	ctx := context.Background()
	
	toolDescriptions, err := mcpClient.ListTools(ctx)
	if err != nil {
		if debug {
			log.Printf("Failed to get tool descriptions, using fallback: %v", err)
		}
		toolDescriptions = "Error: Could not retrieve tool descriptions from MCP server"
	}
	
	systemPrompt := fmt.Sprintf(`You are a world state mutation engine for a text adventure game. 

Your job: Analyze the player's intent and return ONLY the specific world mutations needed.

Available MCP tools:
%s

Return JSON only:
{
  "mutations": [{"tool": "tool_name", "args": {"param": "value"}}],
  "reasoning": "Brief explanation of intent"
}

If no mutations needed, return empty mutations array.`, toolDescriptions)

	req := openai.ChatCompletionRequest{
		Model: "gpt-5-2025-08-07",
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: worldContext + "PLAYER ACTION: " + userInput,
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

func executeMutations(ctx context.Context, mutations []MutationRequest, mcpClient *mcp.WorldStateClient, debug bool) ([]string, []string) {
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
				result, err = mcpClient.AddToInventory(ctx, item)
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

func generateAndExecuteMutationsWithRetries(ctx context.Context, client *openai.Client, userInput string, world game.WorldState, gameHistory []string, mcpClient *mcp.WorldStateClient, debug bool) ([]string, []string, error) {
	const maxRetries = 3
	var allSuccesses []string
	var finalFailures []string
	
	mutationResp, err := generateMutations(client, userInput, world, gameHistory, mcpClient, debug)
	if err != nil {
		return nil, nil, fmt.Errorf("initial mutation generation failed: %w", err)
	}
	
	retryCount := 0
	pendingMutations := mutationResp.Mutations
	
	for retryCount <= maxRetries && len(pendingMutations) > 0 {
		if debug && retryCount > 0 {
			log.Printf("Retry attempt %d with %d mutations", retryCount, len(pendingMutations))
		}
		
		successes, failures := executeMutations(ctx, pendingMutations, mcpClient, debug)
		
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
		
		retryResp, err := generateMutations(client, retryPrompt, world, gameHistory, mcpClient, debug)
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

func startTwoStepLLMFlow(client *openai.Client, userInput string, world game.WorldState, gameHistory []string, logger *logging.CompletionLogger, mcpClient *mcp.WorldStateClient, debug bool) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		
		if debug {
			log.Printf("Starting two-step flow for input: %q", userInput)
		}
		
		successes, failures, err := generateAndExecuteMutationsWithRetries(ctx, client, userInput, world, gameHistory, mcpClient, debug)
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
		
		var allMessages []string
		if debug {
			allMessages = append(allMessages, successes...)
			allMessages = append(allMessages, failures...)
		}
		
		return mutationsGeneratedMsg{
			mutations: allMessages,
			successes: successes,
			failures:  failures,
			newWorld:  newWorld,
			userInput: userInput,
			debug:     debug,
		}
	}
}

func generateNPCThoughts(client *openai.Client, npcID string, world game.WorldState, gameHistory []string, debug bool) tea.Cmd {
	return func() tea.Msg {
		if debug {
			worldContext := buildWorldContext(world, gameHistory)
			
			if debug {
				log.Printf("=== NPC BRAIN: %s ===", npcID)
				log.Printf("World context sent to %s:", npcID)
				log.Printf("%s", worldContext)
				log.Printf("=== END NPC CONTEXT ===")
			}
			
			systemPrompt := fmt.Sprintf(`You are %s, an NPC observing a player. Generate realistic internal thoughts - short, fragmented, like real human thinking.

Your thoughts should be:
- Brief and natural (10-20 words total)  
- Single line, not multiple sentences
- Immediate reactions, not flowery observations
- Simple language, not poetic
- Like actual inner monologue

Examples:
"Hmm, they're looking around carefully."
"Wonder what they want with that key."
"Someone's being cautious. Good."

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
				return npcThoughtsMsg{
					npcID:    npcID,
					thoughts: fmt.Sprintf("*%s seems distracted*", npcID),
					debug:    debug,
				}
			}

			thoughts := resp.Choices[0].Message.Content
			if debug {
				log.Printf("NPC %s generated thoughts: %s", npcID, thoughts)
			}
			
			return npcThoughtsMsg{
				npcID:    npcID,
				thoughts: thoughts,
				debug:    debug,
			}
		}
		
		return npcThoughtsMsg{
			npcID:    npcID,
			thoughts: "",
			debug:    debug,
		}
	}
}