package llm

import (
	"context"
	"fmt"
	"time"

	"github.com/sashabaranov/go-openai"
	"textadventure/internal/game"
	"textadventure/internal/logging"
)

type Client struct {
	openai *openai.Client
	debug  bool
}

func NewClient(apiKey string, debug bool) *Client {
	return &Client{
		openai: openai.NewClient(apiKey),
		debug:  debug,
	}
}

type CompletionRequest struct {
	UserInput    string
	World        game.WorldState
	History      *game.History
	Logger       *logging.CompletionLogger
}

func (c *Client) CreateCompletionStream(ctx context.Context, req CompletionRequest) (*openai.ChatCompletionStream, error) {
	worldContext := req.History.BuildContext(req.World)
	systemPrompt := `You are both narrator and world simulator for a text adventure game. You have complete knowledge of the world state.

Your job: Respond to player actions with 2-4 sentence vivid narration that feels natural and immersive.

Rules:
- Stay consistent with the provided world state
- If action is impossible, explain why and suggest alternatives
- Keep responses concise but atmospheric
- Don't change the world state (that comes later)
- Respond as if you can see everything in the current location`

	openaiReq := openai.ChatCompletionRequest{
		Model: "gpt-5-2025-08-07",
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: worldContext + "PLAYER ACTION: " + req.UserInput,
			},
		},
		MaxCompletionTokens: 200,
		ReasoningEffort:     "minimal",
		Stream:              true,
	}

	stream, err := c.openai.CreateChatCompletionStream(ctx, openaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create completion stream: %w", err)
	}

	return stream, nil
}

func (c *Client) LogCompletion(
	world game.WorldState,
	userInput string,
	systemPrompt string,
	response string,
	startTime time.Time,
	logger *logging.CompletionLogger,
) error {
	if logger == nil {
		return nil
	}

	responseTime := time.Since(startTime)
	metadata := logging.CompletionMetadata{
		Model:         "gpt-5-2025-08-07",
		MaxTokens:     200,
		ResponseTime:  responseTime,
		StreamingUsed: true,
	}

	return logger.LogCompletion(world, userInput, systemPrompt, response, metadata)
}