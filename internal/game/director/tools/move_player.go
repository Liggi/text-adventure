package tools

import (
	"context"
	"fmt"

	"textadventure/internal/game"
	"textadventure/internal/mcp"
)

type MovePlayerTool struct{}

func (t *MovePlayerTool) Name() string {
	return "move_player"
}

func (t *MovePlayerTool) Validate(args map[string]interface{}) error {
	location, ok := args["location"].(string)
	if !ok || location == "" {
		return fmt.Errorf("move_player requires 'location' parameter")
	}
	return nil
}

func (t *MovePlayerTool) Execute(ctx context.Context, args map[string]interface{}, client *mcp.WorldStateClient, world game.WorldState, actingNPCID string) error {
	location := args["location"].(string)
	_, err := client.MovePlayer(ctx, location)
	return err
}

func (t *MovePlayerTool) SuccessMessage(args map[string]interface{}, actingNPCID string) string {
	location := args["location"].(string)
	return fmt.Sprintf("Moved to %s", location)
}