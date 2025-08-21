package tools

import (
	"context"

	"textadventure/internal/game"
	"textadventure/internal/mcp"
)

type GetWorldStateTool struct{}

func (t *GetWorldStateTool) Name() string {
	return "get_world_state"
}

func (t *GetWorldStateTool) Validate(args map[string]interface{}) error {
	return nil
}

func (t *GetWorldStateTool) Execute(ctx context.Context, args map[string]interface{}, client *mcp.WorldStateClient, world game.WorldState, actingNPCID string) error {
	_, err := client.GetWorldState(ctx)
	return err
}

func (t *GetWorldStateTool) SuccessMessage(args map[string]interface{}, actingNPCID string) string {
	return "Retrieved world state"
}