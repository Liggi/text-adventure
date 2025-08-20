package narration

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sashabaranov/go-openai"

	"textadventure/internal/game"
	"textadventure/internal/game/sensory"
	"textadventure/internal/logging"
)

// StreamStartedMsg represents a started narration stream
type StreamStartedMsg struct {
	Stream        *openai.ChatCompletionStream
	Debug         bool
	World         game.WorldState
	UserInput     string
	SystemPrompt  string
	StartTime     time.Time
	Logger        *logging.CompletionLogger
	SensoryEvents *sensory.SensoryEventResponse
}

// StreamChunkMsg represents a chunk from the narration stream
type StreamChunkMsg struct {
	Chunk         string
	Stream        *openai.ChatCompletionStream
	Debug         bool
	CompletionCtx *StreamStartedMsg
}

// StreamCompleteMsg represents completion of narration stream
type StreamCompleteMsg struct {
	World         game.WorldState
	UserInput     string
	SystemPrompt  string
	Response      string
	StartTime     time.Time
	Logger        *logging.CompletionLogger
	Debug         bool
	SensoryEvents *sensory.SensoryEventResponse
}

// StartLLMStream initiates a streaming narration response
func StartLLMStream(client *openai.Client, userInput string, world game.WorldState, gameHistory []string, logger *logging.CompletionLogger, debug bool, mutationResults []string, sensoryEvents *sensory.SensoryEventResponse, actingNPCID ...string) tea.Cmd {
	return func() tea.Msg {
		if debug {
			log.Printf("Starting LLM stream with input: %q", userInput)
		}
		
		startTime := time.Now()
		worldContext := BuildWorldContext(world, gameHistory, actingNPCID...)
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
			return StreamErrorMsg{Response: "", Err: err}
		}
		
		return StreamStartedMsg{
			Stream:        stream,
			Debug:         debug,
			World:         world,
			UserInput:     userInput,
			SystemPrompt:  systemPrompt,
			StartTime:     startTime,
			Logger:        logger,
			SensoryEvents: sensoryEvents,
		}
	}
}

// ReadNextChunk reads the next chunk from the narration stream
func ReadNextChunk(stream *openai.ChatCompletionStream, debug bool, completionCtx *StreamStartedMsg, fullResponse string) tea.Cmd {
	return func() tea.Msg {
		response, err := stream.Recv()
		
		if errors.Is(err, io.EOF) {
			if debug {
				log.Println("Stream finished")
			}
			stream.Close()
			
			responseTime := time.Since(completionCtx.StartTime)
			metadata := logging.CompletionMetadata{
				Model:         "gpt-5-2025-08-07",
				MaxTokens:     200,
				ResponseTime:  responseTime,
				StreamingUsed: true,
			}
			
			if logErr := completionCtx.Logger.LogCompletion(completionCtx.World, completionCtx.UserInput, completionCtx.SystemPrompt, fullResponse, metadata); logErr != nil && debug {
				log.Printf("Failed to log completion: %v", logErr)
			}
			
			return StreamCompleteMsg{
				World:         completionCtx.World,
				UserInput:     completionCtx.UserInput,
				SystemPrompt:  completionCtx.SystemPrompt,
				Response:      fullResponse,
				StartTime:     completionCtx.StartTime,
				Logger:        completionCtx.Logger,
				Debug:         debug,
				SensoryEvents: completionCtx.SensoryEvents,
			}
		}
		
		if err != nil {
			if debug {
				log.Printf("Stream error: %v", err)
			}
			stream.Close()
			return StreamErrorMsg{Response: "", Err: err}
		}
		
		if len(response.Choices) > 0 && response.Choices[0].Delta.Content != "" {
			chunk := response.Choices[0].Delta.Content
			if debug {
				log.Printf("Stream chunk: %q", chunk)
			}
			return StreamChunkMsg{Chunk: chunk, Stream: stream, Debug: debug, CompletionCtx: completionCtx}
		}
		
		return ReadNextChunk(stream, debug, completionCtx, fullResponse)()
	}
}

// StreamErrorMsg represents a streaming error
type StreamErrorMsg struct {
	Response string
	Err      error
}

// BuildWorldContext constructs context string for LLM narration
func BuildWorldContext(world game.WorldState, gameHistory []string, actingNPCID ...string) string {
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