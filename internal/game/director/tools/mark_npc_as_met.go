package tools

import (
	"context"
	"fmt"

	"textadventure/internal/game"
	"textadventure/internal/mcp"
)

type MarkNPCAsMetTool struct{}

func (t *MarkNPCAsMetTool) Name() string {
	return "mark_npc_as_met"
}

func (t *MarkNPCAsMetTool) Validate(args map[string]interface{}) error {
	npcID, ok := args["npc_id"].(string)
	if !ok || npcID == "" {
		return fmt.Errorf("mark_npc_as_met requires 'npc_id' parameter")
	}
	return nil
}

func (t *MarkNPCAsMetTool) Execute(ctx context.Context, args map[string]interface{}, client *mcp.WorldStateClient, world game.WorldState, actingNPCID string) error {
	npcID := args["npc_id"].(string)
	_, err := client.MarkNPCAsMetMethod(ctx, npcID)
	return err
}

func (t *MarkNPCAsMetTool) SuccessMessage(args map[string]interface{}, actingNPCID string) string {
	npcID := args["npc_id"].(string)
	return "Player has now met " + npcID
}