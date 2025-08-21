package tools

import (
	"context"
	"fmt"

	"textadventure/internal/game"
	"textadventure/internal/mcp"
)

type AddToInventoryTool struct{}

func (t *AddToInventoryTool) Name() string {
	return "add_to_inventory"
}

func (t *AddToInventoryTool) Validate(args map[string]interface{}) error {
	item, ok := args["item"].(string)
	if !ok || item == "" {
		return fmt.Errorf("add_to_inventory requires 'item' parameter")
	}
	return nil
}

func (t *AddToInventoryTool) Execute(ctx context.Context, args map[string]interface{}, client *mcp.WorldStateClient, world game.WorldState, actingNPCID string) error {
	item := args["item"].(string)
	_, err := client.AddToInventory(ctx, item)
	return err
}

func (t *AddToInventoryTool) SuccessMessage(args map[string]interface{}, actingNPCID string) string {
	item := args["item"].(string)
	return fmt.Sprintf("Added %s to inventory", item)
}