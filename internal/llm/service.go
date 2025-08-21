package llm

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
	"textadventure/internal/debug"
)

type Service struct {
	client *openai.Client
	model  string
	debug  *debug.Logger
}

func NewService(apiKey string, debug *debug.Logger) *Service {
	return &Service{
		client: openai.NewClient(apiKey),
		model:  "gpt-5-2025-08-07",
		debug:  debug,
	}
}

type TextCompletionRequest struct {
	SystemPrompt string
	UserPrompt   string
	MaxTokens    int
}

type JSONCompletionRequest struct {
	SystemPrompt string
	UserPrompt   string
	MaxTokens    int
}

type StreamCompletionRequest struct {
	SystemPrompt string
	UserPrompt   string
	MaxTokens    int
}

func (s *Service) CompleteText(ctx context.Context, req TextCompletionRequest) (string, error) {
	openaiReq := openai.ChatCompletionRequest{
		Model: s.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: req.SystemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: req.UserPrompt,
			},
		},
		MaxCompletionTokens: req.MaxTokens,
		ReasoningEffort:     "minimal",
	}

	if s.debug != nil {
		s.debug.Printf("LLM Text Completion - MaxTokens: %d, SystemPrompt length: %d", req.MaxTokens, len(req.SystemPrompt))
	}

	resp, err := s.client.CreateChatCompletion(ctx, openaiReq)
	if err != nil {
		if s.debug != nil {
			s.debug.Printf("LLM Text Completion error: %v", err)
		}
		return "", fmt.Errorf("text completion failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no completion choices returned")
	}

	content := resp.Choices[0].Message.Content
	if s.debug != nil {
		s.debug.Printf("LLM Text Completion response length: %d", len(content))
	}

	return content, nil
}

func (s *Service) CompleteJSON(ctx context.Context, req JSONCompletionRequest) (string, error) {
	openaiReq := openai.ChatCompletionRequest{
		Model: s.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: req.SystemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: req.UserPrompt,
			},
		},
		MaxCompletionTokens: req.MaxTokens,
		ReasoningEffort:     "minimal",
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	}

	if s.debug != nil {
		s.debug.Printf("LLM JSON Completion - MaxTokens: %d, SystemPrompt length: %d", req.MaxTokens, len(req.SystemPrompt))
	}

	resp, err := s.client.CreateChatCompletion(ctx, openaiReq)
	if err != nil {
		if s.debug != nil {
			s.debug.Printf("LLM JSON Completion error: %v", err)
		}
		return "", fmt.Errorf("JSON completion failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no completion choices returned")
	}

	content := resp.Choices[0].Message.Content
	if s.debug != nil {
		s.debug.Printf("LLM JSON Completion response length: %d", len(content))
	}

	return content, nil
}

func (s *Service) CompleteStream(ctx context.Context, req StreamCompletionRequest) (*openai.ChatCompletionStream, error) {
	openaiReq := openai.ChatCompletionRequest{
		Model: s.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: req.SystemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: req.UserPrompt,
			},
		},
		MaxCompletionTokens: req.MaxTokens,
		ReasoningEffort:     "minimal",
		Stream:              true,
	}

	if s.debug != nil {
		s.debug.Printf("LLM Stream Completion - MaxTokens: %d, SystemPrompt length: %d", req.MaxTokens, len(req.SystemPrompt))
	}

	stream, err := s.client.CreateChatCompletionStream(ctx, openaiReq)
	if err != nil {
		if s.debug != nil {
			s.debug.Printf("LLM Stream Completion error: %v", err)
		}
		return nil, fmt.Errorf("stream completion failed: %w", err)
	}

	return stream, nil
}