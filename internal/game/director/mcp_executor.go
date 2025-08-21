package director

import (
	"context"
	"fmt"

	"textadventure/internal/debug"
	"textadventure/internal/game"
	"textadventure/internal/mcp"
)

type MutationRequest struct {
	Tool string                 `json:"tool"`
	Args map[string]interface{} `json:"args"`
}

func ExecuteMutations(ctx context.Context, mutations []MutationRequest, mcpClient *mcp.WorldStateClient, debugLogger *debug.Logger, world game.WorldState, actingNPCID string) ([]string, []string) {
	var successes []string
	var failures []string
	
	for _, mutation := range mutations {
		debugLogger.Printf("Executing mutation: %s with args: %v", mutation.Tool, mutation.Args)
		
		tool, exists := GetTool(mutation.Tool)
		if !exists {
			failure := fmt.Sprintf("Unknown tool: %s", mutation.Tool)
			failures = append(failures, failure)
			debugLogger.Printf("ERROR: %s", failure)
			continue
		}
		
		if err := tool.Validate(mutation.Args); err != nil {
			failure := fmt.Sprintf("Invalid args for %s: %v", mutation.Tool, err)
			failures = append(failures, failure)
			debugLogger.Printf("ERROR: %s", failure)
			continue
		}
		
		if err := tool.Execute(ctx, mutation.Args, mcpClient, world, actingNPCID); err != nil {
			failure := fmt.Sprintf("Failed to execute %s: %v", mutation.Tool, err)
			failures = append(failures, failure)
			debugLogger.Printf("ERROR: %s", failure)
		} else {
			success := tool.SuccessMessage(mutation.Args, actingNPCID)
			successes = append(successes, success)
			debugLogger.Printf("SUCCESS: %s", success)
		}
	}
	
	debugLogger.Printf("Mutation execution complete: %d successes, %d failures", len(successes), len(failures))
	
	return successes, failures
}
