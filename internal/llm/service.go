package llm

import (
    "context"
    "fmt"
    "time"
    "strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/ssestream"
	"github.com/openai/openai-go/shared"
	"github.com/openai/openai-go/shared/constant"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"textadventure/internal/debug"
	"textadventure/internal/observability"
)

// Context keys for operation tracing
type contextKey string

const (
	operationTypeKey contextKey = "operation_type"
	gameContextKey   contextKey = "game_context"
	sessionIDKey     contextKey = "session_id"
)

type Service struct {
	client *openai.Client
	model  string
	debug  *debug.Logger
	tracer trace.Tracer
}

func NewService(apiKey string, debug *debug.Logger) *Service {
    client := openai.NewClient(option.WithAPIKey(apiKey))
    return &Service{
		client: &client,
		model:  "gpt-5-2025-08-07",
		debug:  debug,
		tracer: otel.Tracer("llm-service"),
	}
}

type TextCompletionRequest struct {
    SystemPrompt    string
    UserPrompt      string
    MaxTokens       int
    Model           string // optional override
    ReasoningEffort string // optional: minimal, low, medium, high
}

type JSONCompletionRequest struct {
    SystemPrompt    string
    UserPrompt      string
    MaxTokens       int
    Model           string // optional override
    ReasoningEffort string // optional: minimal, low, medium, high
}

type StreamCompletionRequest struct {
    SystemPrompt    string
    UserPrompt      string
    MaxTokens       int
    Model           string // optional override
    ReasoningEffort string // optional: minimal, low, medium, high
}

type JSONSchemaCompletionRequest struct {
    SystemPrompt    string
    UserPrompt      string
    MaxTokens       int
    Model           string // optional override
    ReasoningEffort string // optional: minimal, low, medium, high
    SchemaName      string
    Schema          interface{}
}

func (s *Service) CompleteText(ctx context.Context, req TextCompletionRequest) (string, error) {
    operationType := "text_completion"
    if opType := getOperationType(ctx); opType != "" {
        operationType = opType
    }
	
	sc := trace.SpanFromContext(ctx).SpanContext()
	if s.debug != nil {
		if !sc.IsValid() {
			s.debug.Printf("NO PARENT: ctx missing active span for %s", operationType)
		} else {
			s.debug.Printf("CompleteText trace=%s parentSpan=%s op=%s", sc.TraceID(), sc.SpanID(), operationType)
		}
	}
	
    spanName := operationType
    if spanName == "" {
        spanName = "llm.complete_text"
    }
    model := s.model
    if strings.TrimSpace(req.Model) != "" {
        model = req.Model
    }
    ctx, span := s.tracer.Start(ctx, spanName,
        trace.WithSpanKind(trace.SpanKindClient),
        trace.WithAttributes(
            observability.CreateGenAIAttributes("openai", model, 0, 0, 0.0)...,
        ),
    )
	defer span.End()

	attrs := []attribute.KeyValue{
		attribute.Int("gen_ai.request.max_tokens", req.MaxTokens),
		attribute.String("langfuse.observation.type", "generation"),
		attribute.String("game.operation_type", operationType),
	}
	
	if sessionID := getSessionID(ctx); sessionID != "" {
		attrs = append(attrs, 
			attribute.String("langfuse.session.id", sessionID),
			attribute.String("session.id", sessionID),
		)
	}
	
	if gameCtx := getGameContext(ctx); gameCtx != nil {
		for k, v := range gameCtx {
			switch val := v.(type) {
			case string:
				attrs = append(attrs, attribute.String("game."+k, val))
			case int:
				attrs = append(attrs, attribute.Int("game."+k, val))
			case []string:
				attrs = append(attrs, attribute.StringSlice("game."+k, val))
			}
		}
	}
	
	span.SetAttributes(attrs...)

	span.AddEvent("gen_ai.user.message", trace.WithAttributes(
		attribute.String("gen_ai.system", "openai"),
		attribute.String("content", req.UserPrompt),
	))

	startTime := time.Now()

    openaiReq := openai.ChatCompletionNewParams{
        Model: shared.ChatModel(model),
        Messages: []openai.ChatCompletionMessageParamUnion{
            openai.SystemMessage(req.SystemPrompt),
            openai.UserMessage(req.UserPrompt),
        },
        MaxCompletionTokens: openai.Int(int64(req.MaxTokens)),
    }
    
    if req.ReasoningEffort != "" {
        openaiReq.ReasoningEffort = shared.ReasoningEffort(req.ReasoningEffort)
    }

	if s.debug != nil {
		s.debug.Printf("LLM Text Completion - MaxTokens: %d, SystemPrompt length: %d", req.MaxTokens, len(req.SystemPrompt))
	}

	resp, err := s.client.Chat.Completions.New(ctx, openaiReq)
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
	
	if s.debug != nil {
		s.debug.Printf("JSON Response Debug: content=%q, finish_reason=%s, choices_count=%d", 
			content, resp.Choices[0].FinishReason, len(resp.Choices))
	}

    span.SetAttributes(
        attribute.Int64("gen_ai.usage.input_tokens", resp.Usage.PromptTokens),
        attribute.Int64("gen_ai.usage.output_tokens", resp.Usage.CompletionTokens),
        attribute.Int64("response_time_ms", duration.Milliseconds()),
        attribute.String("langfuse.observation.input", req.SystemPrompt+"\n\n"+req.UserPrompt),
        attribute.String("langfuse.observation.output", content),
        attribute.String("langfuse.observation.output_format", "text"),
        attribute.String("langfuse.observation.model.name", model),
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
    operationType := "json_completion"
    if opType := getOperationType(ctx); opType != "" {
        operationType = opType
    }
	
	sc := trace.SpanFromContext(ctx).SpanContext()
	if s.debug != nil {
		if !sc.IsValid() {
			s.debug.Printf("NO PARENT: ctx missing active span for %s", operationType)
		} else {
			s.debug.Printf("CompleteJSON trace=%s parentSpan=%s op=%s", sc.TraceID(), sc.SpanID(), operationType)
		}
	}
	
    spanName := operationType
    if spanName == "" {
        spanName = "llm.complete_json"
    }
    model := s.model
    if strings.TrimSpace(req.Model) != "" {
        model = req.Model
    }
    ctx, span := s.tracer.Start(ctx, spanName,
        trace.WithSpanKind(trace.SpanKindClient),
        trace.WithAttributes(
            observability.CreateGenAIAttributes("openai", model, 0, 0, 0.0)...,
        ),
    )
	defer span.End()

	attrs := []attribute.KeyValue{
		attribute.Int("gen_ai.request.max_tokens", req.MaxTokens),
		attribute.String("langfuse.observation.type", "generation"),
		attribute.String("response_format", "json"),
		attribute.String("game.operation_type", operationType),
	}
	
	if sessionID := getSessionID(ctx); sessionID != "" {
		attrs = append(attrs, 
			attribute.String("langfuse.session.id", sessionID),
			attribute.String("session.id", sessionID),
		)
	}
	
	if gameCtx := getGameContext(ctx); gameCtx != nil {
		for k, v := range gameCtx {
			switch val := v.(type) {
			case string:
				attrs = append(attrs, attribute.String("game."+k, val))
			case int:
				attrs = append(attrs, attribute.Int("game."+k, val))
			case []string:
				attrs = append(attrs, attribute.StringSlice("game."+k, val))
			}
		}
	}
	
	span.SetAttributes(attrs...)

	span.AddEvent("gen_ai.user.message", trace.WithAttributes(
		attribute.String("gen_ai.system", "openai"),
		attribute.String("content", req.UserPrompt),
	))

	startTime := time.Now()

    openaiReq := openai.ChatCompletionNewParams{
        Model: shared.ChatModel(model),
        Messages: []openai.ChatCompletionMessageParamUnion{
            openai.SystemMessage(req.SystemPrompt),
            openai.UserMessage(req.UserPrompt),
        },
        MaxCompletionTokens: openai.Int(int64(req.MaxTokens)),
        ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
            OfJSONObject: func() *shared.ResponseFormatJSONObjectParam {
                p := shared.NewResponseFormatJSONObjectParam()
                return &p
            }(),
        },
    }
    
    if req.ReasoningEffort != "" {
        openaiReq.ReasoningEffort = shared.ReasoningEffort(req.ReasoningEffort)
    }

	if s.debug != nil {
		s.debug.Printf("LLM JSON Completion - MaxTokens: %d, SystemPrompt length: %d", req.MaxTokens, len(req.SystemPrompt))
		s.debug.Printf("LLM JSON Request - MaxCompletionTokens param: %v", openaiReq.MaxCompletionTokens)
		s.debug.Printf("LLM JSON Request - Model: %v", openaiReq.Model)
		s.debug.Printf("LLM JSON Request - ResponseFormat: %+v", openaiReq.ResponseFormat)
	}

	resp, err := s.client.Chat.Completions.New(ctx, openaiReq)
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
	
	if s.debug != nil {
		s.debug.Printf("JSON Response Debug: content=%q, finish_reason=%s, choices_count=%d", 
			content, resp.Choices[0].FinishReason, len(resp.Choices))
		if resp.Choices[0].FinishReason == "length" {
			s.debug.Printf("JSON Length Debug: input_tokens=%d, completion_tokens=%d, total_available=%d", 
				resp.Usage.PromptTokens, resp.Usage.CompletionTokens, req.MaxTokens)
			s.debug.Printf("JSON Length Debug: message_refusal=%q", resp.Choices[0].Message.Refusal)
		}
	}

    span.SetAttributes(
        attribute.Int64("gen_ai.usage.input_tokens", resp.Usage.PromptTokens),
        attribute.Int64("gen_ai.usage.output_tokens", resp.Usage.CompletionTokens),
        attribute.Int64("response_time_ms", duration.Milliseconds()),
        attribute.String("langfuse.observation.input", req.SystemPrompt+"\n\n"+req.UserPrompt),
        attribute.String("langfuse.observation.output", content),
        attribute.String("langfuse.observation.output_format", "json"),
        attribute.String("langfuse.observation.model.name", model),
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

func (s *Service) CompleteJSONSchema(ctx context.Context, req JSONSchemaCompletionRequest) (string, error) {
    operationType := "json_schema_completion"
    if opType := getOperationType(ctx); opType != "" {
        operationType = opType
    }
	
	sc := trace.SpanFromContext(ctx).SpanContext()
	if s.debug != nil {
		if !sc.IsValid() {
			s.debug.Printf("NO PARENT: ctx missing active span for %s", operationType)
		} else {
			s.debug.Printf("CompleteJSONSchema trace=%s parentSpan=%s op=%s", sc.TraceID(), sc.SpanID(), operationType)
		}
	}
	
    spanName := operationType
    if spanName == "" {
        spanName = "llm.complete_json_schema"
    }
    model := s.model
    if strings.TrimSpace(req.Model) != "" {
        model = req.Model
    }
    ctx, span := s.tracer.Start(ctx, spanName,
        trace.WithSpanKind(trace.SpanKindClient),
        trace.WithAttributes(
            observability.CreateGenAIAttributes("openai", model, 0, 0, 0.0)...,
        ),
    )
	defer span.End()

	attrs := []attribute.KeyValue{
		attribute.Int("gen_ai.request.max_tokens", req.MaxTokens),
		attribute.String("langfuse.observation.type", "generation"),
		attribute.String("response_format", "json_schema"),
		attribute.String("game.operation_type", operationType),
	}
	
	if sessionID := getSessionID(ctx); sessionID != "" {
		attrs = append(attrs, 
			attribute.String("langfuse.session.id", sessionID),
			attribute.String("session.id", sessionID),
		)
	}
	
	if gameCtx := getGameContext(ctx); gameCtx != nil {
		for k, v := range gameCtx {
			switch val := v.(type) {
			case string:
				attrs = append(attrs, attribute.String("game."+k, val))
			case int:
				attrs = append(attrs, attribute.Int("game."+k, val))
			case []string:
				attrs = append(attrs, attribute.StringSlice("game."+k, val))
			}
		}
	}
	
	span.SetAttributes(attrs...)

	span.AddEvent("gen_ai.user.message", trace.WithAttributes(
		attribute.String("gen_ai.system", "openai"),
		attribute.String("content", req.UserPrompt),
	))

	startTime := time.Now()

    openaiReq := openai.ChatCompletionNewParams{
        Model: shared.ChatModel(model),
        Messages: []openai.ChatCompletionMessageParamUnion{
            openai.SystemMessage(req.SystemPrompt),
            openai.UserMessage(req.UserPrompt),
        },
        MaxCompletionTokens: openai.Int(int64(req.MaxTokens)),
        ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
            OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
                Type: constant.JSONSchema("json_schema"),
                JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
                    Name: req.SchemaName,
                    Schema: req.Schema,
                    Strict: openai.Bool(true),
                },
            },
        },
    }
    
    if req.ReasoningEffort != "" {
        openaiReq.ReasoningEffort = shared.ReasoningEffort(req.ReasoningEffort)
    }

	if s.debug != nil {
		s.debug.Printf("LLM JSON Schema Completion - MaxTokens: %d, Schema: %s", req.MaxTokens, req.SchemaName)
	}

	resp, err := s.client.Chat.Completions.New(ctx, openaiReq)
	if err != nil {
		span.SetAttributes(attribute.String("error.type", "llm_completion_error"))
		span.RecordError(err)
		if s.debug != nil {
			s.debug.Printf("LLM JSON Schema Completion error: %v", err)
		}
		return "", fmt.Errorf("JSON schema completion failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		err := fmt.Errorf("no completion choices returned")
		span.RecordError(err)
		return "", err
	}

	content := resp.Choices[0].Message.Content
	duration := time.Since(startTime)
	
	if s.debug != nil {
		s.debug.Printf("JSON Schema Response: content=%q, finish_reason=%s", 
			content, resp.Choices[0].FinishReason)
	}

    span.SetAttributes(
        attribute.Int64("gen_ai.usage.input_tokens", resp.Usage.PromptTokens),
        attribute.Int64("gen_ai.usage.output_tokens", resp.Usage.CompletionTokens),
        attribute.Int64("response_time_ms", duration.Milliseconds()),
        attribute.String("langfuse.observation.input", req.SystemPrompt+"\n\n"+req.UserPrompt),
        attribute.String("langfuse.observation.output", content),
        attribute.String("langfuse.observation.output_format", "json_schema"),
        attribute.String("langfuse.observation.model.name", model),
    )

	span.AddEvent("gen_ai.choice", trace.WithAttributes(
		attribute.String("gen_ai.system", "openai"),
		attribute.String("content", content),
	))

	if s.debug != nil {
		s.debug.Printf("LLM JSON Schema Completion response length: %d, tokens: %d/%d, duration: %v", 
			len(content), resp.Usage.PromptTokens, resp.Usage.CompletionTokens, duration)
	}

	return content, nil
}

func WithOperationType(ctx context.Context, opType string) context.Context {
	return context.WithValue(ctx, operationTypeKey, opType)
}

func WithGameContext(ctx context.Context, gameCtx map[string]interface{}) context.Context {
    // Merge with any existing game context instead of overwriting
    if existing, ok := ctx.Value(gameContextKey).(map[string]interface{}); ok && existing != nil {
        merged := make(map[string]interface{}, len(existing)+len(gameCtx))
        for k, v := range existing {
            merged[k] = v
        }
        for k, v := range gameCtx {
            merged[k] = v
        }
        return context.WithValue(ctx, gameContextKey, merged)
    }
    return context.WithValue(ctx, gameContextKey, gameCtx)
}

func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, observability.GetSessionIDKey(), sessionID)
}

func getOperationType(ctx context.Context) string {
	if opType, ok := ctx.Value(operationTypeKey).(string); ok {
		return opType
	}
	return ""
}

func getGameContext(ctx context.Context) map[string]interface{} {
	if gameCtx, ok := ctx.Value(gameContextKey).(map[string]interface{}); ok {
		return gameCtx
	}
	return nil
}

func getSessionID(ctx context.Context) string {
    return observability.GetSessionIDFromContext(ctx)
}

// CopyGameContextToSpan attaches game context and session id attributes to an existing span.
func CopyGameContextToSpan(ctx context.Context, span trace.Span) {
    if span == nil {
        return
    }
    if sid := getSessionID(ctx); sid != "" {
        span.SetAttributes(
            attribute.String("langfuse.session.id", sid),
            attribute.String("session.id", sid),
        )
    }
    if gameCtx := getGameContext(ctx); gameCtx != nil {
        for k, v := range gameCtx {
            switch val := v.(type) {
            case string:
                span.SetAttributes(attribute.String("game."+k, val))
            case int:
                span.SetAttributes(attribute.Int("game."+k, val))
            case []string:
                span.SetAttributes(attribute.StringSlice("game."+k, val))
            }
        }
    }
}

func (s *Service) CompleteStream(ctx context.Context, req StreamCompletionRequest) (*ssestream.Stream[openai.ChatCompletionChunk], error) {
    model := s.model
    if strings.TrimSpace(req.Model) != "" {
        model = req.Model
    }
    openaiReq := openai.ChatCompletionNewParams{
        Model: shared.ChatModel(model),
        Messages: []openai.ChatCompletionMessageParamUnion{
            openai.SystemMessage(req.SystemPrompt),
            openai.UserMessage(req.UserPrompt),
        },
        MaxCompletionTokens: openai.Int(int64(req.MaxTokens)),
    }
    
    if req.ReasoningEffort != "" {
        openaiReq.ReasoningEffort = shared.ReasoningEffort(req.ReasoningEffort)
    }

	if s.debug != nil {
		s.debug.Printf("LLM Stream Completion - MaxTokens: %d, SystemPrompt length: %d", req.MaxTokens, len(req.SystemPrompt))
		s.debug.Printf("LLM Stream Request - Model: %s", model)
	}

	stream := s.client.Chat.Completions.NewStreaming(ctx, openaiReq)
	return stream, nil
}
