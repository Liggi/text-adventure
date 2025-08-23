package director

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"textadventure/internal/debug"
	"textadventure/internal/game"
	"textadventure/internal/mcp"
	"textadventure/internal/observability"
)

type MutationRequest struct {
	Tool string                 `json:"tool"`
	Args map[string]interface{} `json:"args"`
}

func ExecuteMutations(ctx context.Context, mutations []MutationRequest, mcpClient *mcp.WorldStateClient, debugLogger *debug.Logger, world game.WorldState, actingNPCID string) ([]string, []string) {
	tracer := otel.Tracer("mcp-executor")
	
	attrs := []attribute.KeyValue{
		attribute.Int("mutation_count", len(mutations)),
		attribute.String("player_location", world.Location),
		attribute.String("acting_npc", actingNPCID),
	}
	
	if sessionID := observability.GetSessionIDFromContext(ctx); sessionID != "" {
		attrs = append(attrs, 
			attribute.String("langfuse.session.id", sessionID),
			attribute.String("session.id", sessionID),
		)
	}
	
	ctx, span := tracer.Start(ctx, "mcp.execute_mutations",
		trace.WithAttributes(attrs...),
	)
	defer span.End()
	
	var successes []string
	var failures []string
	
	for i, mutation := range mutations {
		_, mutSpan := tracer.Start(ctx, "mcp.execute_tool",
			trace.WithAttributes(
				attribute.String("tool_name", mutation.Tool),
				attribute.Int("mutation_index", i),
			),
		)
		
		tool, exists := GetTool(mutation.Tool)
		if !exists {
			failure := fmt.Sprintf("Unknown tool: %s", mutation.Tool)
			failures = append(failures, failure)
			mutSpan.SetAttributes(attribute.String("error_type", "tool_not_found"))
			mutSpan.End()
			continue
		}
		
		if err := tool.Validate(mutation.Args); err != nil {
			failure := fmt.Sprintf("Invalid args for %s: %v", mutation.Tool, err)
			failures = append(failures, failure)
			mutSpan.SetAttributes(attribute.String("error_type", "validation_failed"))
			mutSpan.RecordError(err)
			mutSpan.End()
			continue
		}
		
		if err := tool.Execute(ctx, mutation.Args, mcpClient, world, actingNPCID); err != nil {
			failure := fmt.Sprintf("Failed to execute %s: %v", mutation.Tool, err)
			failures = append(failures, failure)
			mutSpan.SetAttributes(attribute.String("error_type", "execution_failed"))
			mutSpan.RecordError(err)
		} else {
			success := tool.SuccessMessage(mutation.Args, actingNPCID)
			successes = append(successes, success)
			mutSpan.SetAttributes(attribute.String("result", "success"))
		}
		mutSpan.End()
	}
	
	if len(failures) > 0 {
		debugLogger.Printf("%d mutations failed", len(failures))
		span.SetAttributes(
			attribute.Int("failure_count", len(failures)),
			attribute.StringSlice("failures", failures),
		)
	}
	
	span.SetAttributes(
		attribute.Int("success_count", len(successes)),
		attribute.StringSlice("successes", successes),
	)
	
	return successes, failures
}
