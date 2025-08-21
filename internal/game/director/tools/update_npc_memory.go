package tools

import (
	"context"
	"fmt"

	"textadventure/internal/game"
	"textadventure/internal/mcp"
)

type UpdateNPCMemoryTool struct{}

func (t *UpdateNPCMemoryTool) Name() string {
	return "update_npc_memory"
}

func (t *UpdateNPCMemoryTool) Validate(args map[string]interface{}) error {
	npcID, ok := args["npc_id"].(string)
	if !ok || npcID == "" {
		return fmt.Errorf("update_npc_memory requires 'npc_id' parameter")
	}
	return nil
}

func (t *UpdateNPCMemoryTool) Execute(ctx context.Context, args map[string]interface{}, client *mcp.WorldStateClient, world game.WorldState, actingNPCID string) error {
	npcID := args["npc_id"].(string)
	
	thought, _ := args["thought"].(string)
	action, _ := args["action"].(string)
	
	_, err := client.UpdateNPCMemory(ctx, npcID, thought, action)
	return err
}

func (t *UpdateNPCMemoryTool) SuccessMessage(args map[string]interface{}, actingNPCID string) string {
	npcID := args["npc_id"].(string)
	
	updates := []string{}
	if thought, ok := args["thought"].(string); ok && thought != "" {
		updates = append(updates, "thought")
	}
	if action, ok := args["action"].(string); ok && action != "" {
		updates = append(updates, "action")
	}
	
	if len(updates) > 0 {
		return fmt.Sprintf("Updated %s memory (%s)", npcID, updates)
	}
	return fmt.Sprintf("Updated %s memory", npcID)
}