package tools

import (
	"context"
	"fmt"

	"textadventure/internal/game"
	"textadventure/internal/mcp"
)

type UnlockDoorTool struct{}

func (t *UnlockDoorTool) Name() string {
	return "unlock_door"
}

func (t *UnlockDoorTool) Validate(args map[string]interface{}) error {
	fromLoc, hasFrom := args["from_location"].(string)
	toLoc, hasTo := args["to_location"].(string)
	
	if !hasFrom || fromLoc == "" {
		return fmt.Errorf("unlock_door requires 'from_location' parameter")
	}
	if !hasTo || toLoc == "" {
		return fmt.Errorf("unlock_door requires 'to_location' parameter")
	}
	return nil
}

func (t *UnlockDoorTool) Execute(ctx context.Context, args map[string]interface{}, client *mcp.WorldStateClient, world game.WorldState, actingNPCID string) error {
	fromLoc := args["from_location"].(string)
	toLoc := args["to_location"].(string)
	
	_, err := client.UnlockDoor(ctx, fromLoc, toLoc, "")
	return err
}

func (t *UnlockDoorTool) SuccessMessage(args map[string]interface{}, actingNPCID string) string {
	fromLoc := args["from_location"].(string)
	toLoc := args["to_location"].(string)
	
	return fmt.Sprintf("Unlocked door from %s to %s", fromLoc, toLoc)
}