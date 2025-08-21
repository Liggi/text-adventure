package tools

import (
	"context"
	"fmt"

	"textadventure/internal/game"
	"textadventure/internal/mcp"
)

type TransferItemTool struct{}

func (t *TransferItemTool) Name() string {
	return "transfer_item"
}

func (t *TransferItemTool) Validate(args map[string]interface{}) error {
	item, hasItem := args["item"].(string)
	fromLoc, hasFrom := args["from_location"].(string)
	toLoc, hasTo := args["to_location"].(string)
	
	if !hasItem || item == "" {
		return fmt.Errorf("transfer_item requires 'item' parameter")
	}
	if !hasFrom || fromLoc == "" {
		return fmt.Errorf("transfer_item requires 'from_location' parameter")
	}
	if !hasTo || toLoc == "" {
		return fmt.Errorf("transfer_item requires 'to_location' parameter")
	}
	return nil
}

func (t *TransferItemTool) Execute(ctx context.Context, args map[string]interface{}, client *mcp.WorldStateClient, world game.WorldState, actingNPCID string) error {
	item := args["item"].(string)
	fromLoc := args["from_location"].(string)
	toLoc := args["to_location"].(string)
	
	_, err := client.TransferItem(ctx, item, fromLoc, toLoc)
	return err
}

func (t *TransferItemTool) SuccessMessage(args map[string]interface{}, actingNPCID string) string {
	item := args["item"].(string)
	fromLoc := args["from_location"].(string)
	toLoc := args["to_location"].(string)
	
	return fmt.Sprintf("Transferred %s from %s to %s", item, fromLoc, toLoc)
}