package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
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

func startLLMStream(client *openai.Client, userInput string, world game.WorldState, gameHistory []string, logger *logging.CompletionLogger, debug bool) tea.Cmd {
	return func() tea.Msg {
		if debug {
			log.Printf("Starting LLM stream with input: %q", userInput)
		}
		
		startTime := time.Now()
		worldContext := buildWorldContext(world, gameHistory)
		systemPrompt := `You are both narrator and world simulator for a text adventure game. You have complete knowledge of the world state.

Your job: Respond to player actions with 2-4 sentence vivid narration that feels natural and immersive.

Rules:
- Stay consistent with the provided world state
- If action is impossible, explain why and suggest alternatives
- Keep responses concise but atmospheric
- Don't change the world state (that comes later)
- Respond as if you can see everything in the current location`

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

func generateMutations(client *openai.Client, userInput string, world game.WorldState, gameHistory []string, debug bool) (*MutationResponse, error) {
	worldContext := buildWorldContext(world, gameHistory)
	
	systemPrompt := `You are a world state mutation engine for a text adventure game. 

Your job: Analyze the player's intent and return ONLY the specific world mutations needed.

Available MCP tools:
- add_to_inventory: {"item": "item_id"} 
- remove_from_inventory: {"item": "item_id"}
- move_player: {"location": "location_id"}
- transfer_item: {"item": "item_id", "from_location": "source", "to_location": "dest"}
- unlock_door: {"location": "location_id", "direction": "north/south", "key_item": "key_id"}

Return JSON only:
{
  "mutations": [{"tool": "add_to_inventory", "args": {"item": "silver_key"}}],
  "reasoning": "Player wants to pick up the key"
}

If no mutations needed, return empty mutations array.`

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

func startTwoStepLLMFlow(client *openai.Client, userInput string, world game.WorldState, gameHistory []string, logger *logging.CompletionLogger, mcpClient *mcp.WorldStateClient, debug bool) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		
		if debug {
			log.Printf("Starting two-step flow for input: %q", userInput)
		}
		
		mutationResp, err := generateMutations(client, userInput, world, gameHistory, debug)
		if err != nil {
			if debug {
				log.Printf("Mutation generation failed: %v", err)
			}
			return llmResponseMsg{response: "", err: err}
		}
		
		successes, failures := executeMutations(ctx, mutationResp.Mutations, mcpClient, debug)
		
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
			allMessages = append(allMessages, fmt.Sprintf("[DEBUG] Mutation reasoning: %s", mutationResp.Reasoning))
			allMessages = append(allMessages, successes...)
			allMessages = append(allMessages, failures...)
		}
		
		return mutationsGeneratedMsg{
			mutations: allMessages,
			failures:  failures,
			newWorld:  newWorld,
			userInput: userInput,
			debug:     debug,
		}
	}
}