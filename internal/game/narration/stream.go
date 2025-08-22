package narration

import (
	"context"
	"errors"
	"io"
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sashabaranov/go-openai"

	"textadventure/internal/game"
	"textadventure/internal/game/sensory"
	"textadventure/internal/llm"
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
func StartLLMStream(llmService *llm.Service, userInput string, world game.WorldState, gameHistory []string, logger *logging.CompletionLogger, debug bool, actionContext string, mutationResults []string, sensoryEvents *sensory.SensoryEventResponse, actingNPCID ...string) tea.Cmd {
	return func() tea.Msg {
		if debug {
			log.Printf("Starting LLM stream with input: %q", userInput)
		}
		
		startTime := time.Now()
		worldContext := game.BuildWorldContext(world, gameHistory, actingNPCID...)
		systemPrompt := buildNarrationPrompt(actionContext, mutationResults, sensoryEvents)
		
		req := llm.StreamCompletionRequest{
			SystemPrompt: systemPrompt,
			UserPrompt:   worldContext + "PLAYER ACTION: " + userInput,
			MaxTokens:    200,
		}
		
		stream, err := llmService.CompleteStream(context.Background(), req)
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

