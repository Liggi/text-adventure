package tools

import (
	"context"
	"fmt"

	"textadventure/internal/game"
	"textadventure/internal/mcp"
)

type RemoveFromInventoryTool struct{}

func (t *RemoveFromInventoryTool) Name() string {
	return "remove_from_inventory"
}

func (t *RemoveFromInventoryTool) Validate(args map[string]interface{}) error {
	item, ok := args["item"].(string)
	if !ok || item == "" {
		return fmt.Errorf("remove_from_inventory requires 'item' parameter")
	}
	return nil
}

func (t *RemoveFromInventoryTool) Execute(ctx context.Context, args map[string]interface{}, client *mcp.WorldStateClient, world game.WorldState, actingNPCID string) error {
	item := args["item"].(string)
	_, err := client.RemoveFromInventory(ctx, item)
	return err
}

func (t *RemoveFromInventoryTool) SuccessMessage(args map[string]interface{}, actingNPCID string) string {
	item := args["item"].(string)
	return fmt.Sprintf("Removed %s from inventory", item)
}