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
    loc, hasLoc := args["location"].(string)
    dir, hasDir := args["direction"].(string)
    key, hasKey := args["key_item"].(string)

    if !hasLoc || loc == "" {
        return fmt.Errorf("unlock_door requires 'location' parameter")
    }
    if !hasDir || dir == "" {
        return fmt.Errorf("unlock_door requires 'direction' parameter")
    }
    if !hasKey || key == "" {
        return fmt.Errorf("unlock_door requires 'key_item' parameter")
    }
    return nil
}

func (t *UnlockDoorTool) Execute(ctx context.Context, args map[string]interface{}, client *mcp.WorldStateClient, world game.WorldState, actingNPCID string) error {
    loc := args["location"].(string)
    dir := args["direction"].(string)
    key := args["key_item"].(string)

    _, err := client.UnlockDoor(ctx, loc, dir, key)
    return err
}

func (t *UnlockDoorTool) SuccessMessage(args map[string]interface{}, actingNPCID string) string {
    loc := args["location"].(string)
    dir := args["direction"].(string)
    key := args["key_item"].(string)
    return fmt.Sprintf("Unlocked %s door in %s with %s", dir, loc, key)
}
