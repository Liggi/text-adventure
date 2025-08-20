package main

import (
	"context"
	"fmt"
	"os"

	"github.com/sashabaranov/go-openai"

	"textadventure/cmd/game/ui"
	"textadventure/internal/debug"
	"textadventure/internal/logging"
	"textadventure/internal/mcp"
)

func createApp() (ui.Model, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return ui.Model{}, fmt.Errorf("please set OPENAI_API_KEY environment variable")
	}
	
	client := openai.NewClient(apiKey)
	debugMode := os.Getenv("DEBUG") == "1" || os.Getenv("DEBUG") == "true"
	
	debugLogger := debug.NewLogger(debugMode)
	debugLogger.Println("Starting text adventure with debug logging")
	
	logger, err := logging.NewCompletionLogger()
	if err != nil {
		return ui.Model{}, fmt.Errorf("failed to initialize completion logger: %w", err)
	}
	
	debugLogger.Println("Initializing MCP client...")
	mcpClient, err := mcp.NewWorldStateClient(debugMode)
	if err != nil {
		return ui.Model{}, fmt.Errorf("failed to initialize MCP client: %w", err)
	}
	
	ctx := context.Background()
	debugLogger.Println("Connecting to MCP server...")
	if err := mcpClient.Connect(ctx); err != nil {
		return ui.Model{}, fmt.Errorf("failed to connect to MCP server: %w", err)
	}
	
	debugLogger.Println("Fetching initial world state from MCP server...")
	mcpWorld, err := mcpClient.GetWorldState(ctx)
	if err != nil {
		return ui.Model{}, fmt.Errorf("failed to get initial world state: %w", err)
	}
	
	debugLogger.Printf("MCP world: player at %s, inventory: %v", mcpWorld.Player.Location, mcpWorld.Player.Inventory)
	
	world := mcp.MCPToGameWorldState(mcpWorld)
	
	debugLogger.Printf("Game world converted: player at %s, inventory: %v", world.Location, world.Inventory)
	
	model := ui.NewModel(client, world, logger, debugMode)
	model.SetMCPClient(mcpClient)
	
	return model, nil
}