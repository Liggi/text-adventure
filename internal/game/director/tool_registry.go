package director

import (
	"context"

	"textadventure/internal/game"
	"textadventure/internal/game/director/tools"
	"textadventure/internal/mcp"
)

type MCPTool interface {
	Validate(args map[string]interface{}) error
	Execute(ctx context.Context, args map[string]interface{}, client *mcp.WorldStateClient, world game.WorldState, actingNPCID string) error
	SuccessMessage(args map[string]interface{}, actingNPCID string) string
	Name() string
}

var toolRegistry = make(map[string]MCPTool)

func init() {
	RegisterTool(&tools.GetWorldStateTool{})
	RegisterTool(&tools.MovePlayerTool{})
	RegisterTool(&tools.MoveNPCTool{})
	RegisterTool(&tools.TransferItemTool{})
	RegisterTool(&tools.AddToInventoryTool{})
	RegisterTool(&tools.RemoveFromInventoryTool{})
	RegisterTool(&tools.UnlockDoorTool{})
	RegisterTool(&tools.UpdateNPCMemoryTool{})
	RegisterTool(&tools.MarkNPCAsMetTool{})
}

func RegisterTool(tool MCPTool) {
	toolRegistry[tool.Name()] = tool
}

func GetTool(name string) (MCPTool, bool) {
	tool, exists := toolRegistry[name]
	return tool, exists
}