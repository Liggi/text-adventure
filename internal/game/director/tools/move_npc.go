package tools

import (
	"context"
	"fmt"

	"textadventure/internal/game"
	"textadventure/internal/mcp"
)

type MoveNPCTool struct{}

func (t *MoveNPCTool) Name() string {
	return "move_npc"
}

func (t *MoveNPCTool) Validate(args map[string]interface{}) error {
	npcID, hasNPC := args["npc_id"].(string)
	location, hasLocation := args["location"].(string)
	
	if !hasNPC || npcID == "" {
		return fmt.Errorf("move_npc requires 'npc_id' parameter")
	}
	if !hasLocation || location == "" {
		return fmt.Errorf("move_npc requires 'location' parameter")
	}
	return nil
}

func (t *MoveNPCTool) Execute(ctx context.Context, args map[string]interface{}, client *mcp.WorldStateClient, world game.WorldState, actingNPCID string) error {
	npcID := args["npc_id"].(string)
	location := args["location"].(string)
	_, err := client.MoveNPC(ctx, npcID, location)
	return err
}

func (t *MoveNPCTool) SuccessMessage(args map[string]interface{}, actingNPCID string) string {
	npcID := args["npc_id"].(string)
	location := args["location"].(string)
	return fmt.Sprintf("NPC %s moved to %s", npcID, location)
}