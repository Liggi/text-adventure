package observability

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

// Config holds the configuration for OpenTelemetry tracing
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	Enabled        bool
	LangfuseHost   string
	PublicKey      string
	SecretKey      string
}

// TracerProvider wraps the OpenTelemetry tracer provider with cleanup
type TracerProvider struct {
	provider *sdktrace.TracerProvider
	enabled  bool
}

// InitTracing initializes OpenTelemetry tracing with Langfuse export
func InitTracing(ctx context.Context, config Config) (*TracerProvider, error) {
	if !config.Enabled {
		// Return a no-op tracer provider
		return &TracerProvider{enabled: false}, nil
	}
	
	// Create OTLP exporter for Langfuse
	exporter, err := createLangfuseExporter(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Langfuse exporter: %w", err)
	}
	
	// Create resource with service information
	res, err := createResource(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}
	
	// Create tracer provider with batching for efficiency
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter, 
			sdktrace.WithBatchTimeout(5*time.Second),
			sdktrace.WithMaxExportBatchSize(100),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(sessionInjector{}),
		// Sample all traces in development, adjust for production
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	
	// Set global tracer provider
	otel.SetTracerProvider(tp)
	
	return &TracerProvider{
		provider: tp,
		enabled:  true,
	}, nil
}

// GetTracer returns a tracer for the given name
func (tp *TracerProvider) GetTracer(name string, options ...trace.TracerOption) trace.Tracer {
	if !tp.enabled {
		return trace.NewNoopTracerProvider().Tracer(name, options...)
	}
	return otel.Tracer(name, options...)
}

// Shutdown gracefully shuts down the tracer provider
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	if !tp.enabled || tp.provider == nil {
		return nil
	}
	return tp.provider.Shutdown(ctx)
}

// IsEnabled returns whether tracing is enabled
func (tp *TracerProvider) IsEnabled() bool {
	return tp.enabled
}

// createLangfuseExporter creates an OTLP HTTP exporter configured for Langfuse
func createLangfuseExporter(ctx context.Context, config Config) (sdktrace.SpanExporter, error) {
	auth := base64.StdEncoding.EncodeToString([]byte(config.PublicKey + ":" + config.SecretKey))
	
	// Use the full traces endpoint path since we're using WithEndpointURL()
	host := strings.TrimSuffix(config.LangfuseHost, "/")
	baseEndpoint := fmt.Sprintf("%s/api/public/otel/v1/traces", host)
	
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(baseEndpoint),
		otlptracehttp.WithHeaders(map[string]string{
			"Authorization": "Basic " + auth,
		}),
		otlptracehttp.WithCompression(otlptracehttp.GzipCompression),
		otlptracehttp.WithTimeout(30*time.Second),
		otlptracehttp.WithInsecure(),
	)
	
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP HTTP exporter: %w", err)
	}
	
	return exporter, nil
}

// createResource creates an OpenTelemetry resource with service metadata
func createResource(config Config) (*resource.Resource, error) {
	return resource.NewWithAttributes(
		"",
		semconv.ServiceName(config.ServiceName),
		semconv.ServiceVersion(config.ServiceVersion),
		attribute.String("deployment.environment", config.Environment),
	), nil
}

// LoadConfigFromEnv loads tracing configuration from environment variables
func LoadConfigFromEnv() Config {
	enabled := os.Getenv("OTEL_TRACES_ENABLED") == "true"
	
	// If tracing is disabled, return minimal config
	if !enabled {
		return Config{
			ServiceName:    "text-adventure",
			ServiceVersion: "1.0.0",
			Environment:    "development",
			Enabled:        false,
		}
	}
	
	// Default to Langfuse cloud EU if not specified
	langfuseHost := os.Getenv("LANGFUSE_HOST")
	if langfuseHost == "" {
		langfuseHost = "https://cloud.langfuse.com"
	}
	
	environment := os.Getenv("ENVIRONMENT")
	if environment == "" {
		environment = "development"
	}
	
	return Config{
		ServiceName:    "text-adventure",
		ServiceVersion: "1.0.0",
		Environment:    environment,
		Enabled:        enabled,
		LangfuseHost:   langfuseHost,
		PublicKey:      os.Getenv("LANGFUSE_PUBLIC_KEY"),
		SecretKey:      os.Getenv("LANGFUSE_SECRET_KEY"),
	}
}

// CreateLangfuseAttributes creates Langfuse-specific span attributes
func CreateLangfuseAttributes(traceName, sessionID, userID string, tags []string) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("langfuse.trace.name", traceName),
	}
	
	if sessionID != "" {
		attrs = append(attrs, attribute.String("langfuse.session.id", sessionID))
	}
	
	if userID != "" {
		attrs = append(attrs, attribute.String("langfuse.user.id", userID))
	}
	
	if len(tags) > 0 {
		attrs = append(attrs, attribute.StringSlice("langfuse.trace.tags", tags))
	}
	
	return attrs
}

// CreateGenAIAttributes creates GenAI semantic convention attributes for LLM spans
func CreateGenAIAttributes(system, model string, inputTokens, outputTokens int, temperature float64) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("gen_ai.operation.name", "chat"),
		attribute.String("gen_ai.system", system),
		attribute.String("gen_ai.request.model", model),
	}
	
	if inputTokens > 0 {
		attrs = append(attrs, attribute.Int("gen_ai.usage.input_tokens", inputTokens))
	}
	
	if outputTokens > 0 {
		attrs = append(attrs, attribute.Int("gen_ai.usage.output_tokens", outputTokens))
	}
	
	if temperature >= 0 {
		attrs = append(attrs, attribute.Float64("gen_ai.request.temperature", temperature))
	}
	
	return attrs
}

type sessionInjector struct{}

func (sessionInjector) OnStart(ctx context.Context, s sdktrace.ReadWriteSpan) {
	if sid := GetSessionIDFromContext(ctx); sid != "" {
		s.SetAttributes(
			attribute.String("langfuse.session.id", sid),
			attribute.String("session.id", sid),
		)
	}
}

type contextKey string
const sessionIDKey contextKey = "session_id"

func GetSessionIDFromContext(ctx context.Context) string {
	if sessionID, ok := ctx.Value(sessionIDKey).(string); ok {
		return sessionID
	}
	return ""
}

func GetSessionIDKey() contextKey {
	return sessionIDKey
}

func (sessionInjector) OnEnd(s sdktrace.ReadOnlySpan) {}
func (sessionInjector) Shutdown(context.Context) error { return nil }
func (sessionInjector) ForceFlush(context.Context) error { return nil }