package llm

import (
	"context"
	"fmt"
	"time"

	"github.com/sashabaranov/go-openai"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"textadventure/internal/debug"
	"textadventure/internal/observability"
)

type Service struct {
	client *openai.Client
	model  string
	debug  *debug.Logger
	tracer trace.Tracer
}

func NewService(apiKey string, debug *debug.Logger) *Service {
	return &Service{
		client: openai.NewClient(apiKey),
		model:  "gpt-5-2025-08-07",
		debug:  debug,
		tracer: otel.Tracer("llm-service"),
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
	ctx, span := s.tracer.Start(ctx, "llm.complete_text",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			observability.CreateGenAIAttributes("openai", s.model, 0, 0, 0.0)...,
		),
	)
	defer span.End()

	span.SetAttributes(
		attribute.Int("gen_ai.request.max_tokens", req.MaxTokens),
		attribute.String("langfuse.observation.type", "generation"),
	)

	span.AddEvent("gen_ai.user.message", trace.WithAttributes(
		attribute.String("gen_ai.system", "openai"),
		attribute.String("content", req.UserPrompt),
	))

	startTime := time.Now()

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
		span.SetAttributes(attribute.String("error.type", "llm_completion_error"))
		span.RecordError(err)
		if s.debug != nil {
			s.debug.Printf("LLM Text Completion error: %v", err)
		}
		return "", fmt.Errorf("text completion failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		err := fmt.Errorf("no completion choices returned")
		span.RecordError(err)
		return "", err
	}

	content := resp.Choices[0].Message.Content
	duration := time.Since(startTime)

	span.SetAttributes(
		attribute.Int("gen_ai.usage.input_tokens", resp.Usage.PromptTokens),
		attribute.Int("gen_ai.usage.output_tokens", resp.Usage.CompletionTokens),
		attribute.Int64("response_time_ms", duration.Milliseconds()),
		attribute.String("langfuse.observation.input", req.SystemPrompt+"\n\n"+req.UserPrompt),
		attribute.String("langfuse.observation.output", content),
		attribute.String("langfuse.observation.model.name", s.model),
	)

	span.AddEvent("gen_ai.choice", trace.WithAttributes(
		attribute.String("gen_ai.system", "openai"),
		attribute.String("content", content),
	))

	if s.debug != nil {
		s.debug.Printf("LLM Text Completion response length: %d, tokens: %d/%d, duration: %v", 
			len(content), resp.Usage.PromptTokens, resp.Usage.CompletionTokens, duration)
	}

	return content, nil
}

func (s *Service) CompleteJSON(ctx context.Context, req JSONCompletionRequest) (string, error) {
	ctx, span := s.tracer.Start(ctx, "llm.complete_json",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			observability.CreateGenAIAttributes("openai", s.model, 0, 0, 0.0)...,
		),
	)
	defer span.End()

	span.SetAttributes(
		attribute.Int("gen_ai.request.max_tokens", req.MaxTokens),
		attribute.String("langfuse.observation.type", "generation"),
		attribute.String("response_format", "json"),
	)

	span.AddEvent("gen_ai.user.message", trace.WithAttributes(
		attribute.String("gen_ai.system", "openai"),
		attribute.String("content", req.UserPrompt),
	))

	startTime := time.Now()

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
		span.SetAttributes(attribute.String("error.type", "llm_completion_error"))
		span.RecordError(err)
		if s.debug != nil {
			s.debug.Printf("LLM JSON Completion error: %v", err)
		}
		return "", fmt.Errorf("JSON completion failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		err := fmt.Errorf("no completion choices returned")
		span.RecordError(err)
		return "", err
	}

	content := resp.Choices[0].Message.Content
	duration := time.Since(startTime)

	span.SetAttributes(
		attribute.Int("gen_ai.usage.input_tokens", resp.Usage.PromptTokens),
		attribute.Int("gen_ai.usage.output_tokens", resp.Usage.CompletionTokens),
		attribute.Int64("response_time_ms", duration.Milliseconds()),
		attribute.String("langfuse.observation.input", req.SystemPrompt+"\n\n"+req.UserPrompt),
		attribute.String("langfuse.observation.output", content),
		attribute.String("langfuse.observation.model.name", s.model),
	)

	span.AddEvent("gen_ai.choice", trace.WithAttributes(
		attribute.String("gen_ai.system", "openai"),
		attribute.String("content", content),
	))

	if s.debug != nil {
		s.debug.Printf("LLM JSON Completion response length: %d, tokens: %d/%d, duration: %v", 
			len(content), resp.Usage.PromptTokens, resp.Usage.CompletionTokens, duration)
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