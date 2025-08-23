package main

import (
	"context"
	"fmt"
	"os"

	"textadventure/cmd/game/ui"
	"textadventure/internal/debug"
	"textadventure/internal/llm"
	"textadventure/internal/logging"
	"textadventure/internal/mcp"
	"textadventure/internal/observability"
)

func createApp() (ui.Model, func(), error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return ui.Model{}, nil, fmt.Errorf("please set OPENAI_API_KEY environment variable")
	}
	
	debugMode := os.Getenv("DEBUG") == "1" || os.Getenv("DEBUG") == "true"
	
	debugLogger := debug.NewLogger(debugMode)
	
	ctx := context.Background()
	tracingConfig := observability.LoadConfigFromEnv()
	tracerProvider, err := observability.InitTracing(ctx, tracingConfig)
	if err != nil {
		debugLogger.Printf("Failed to initialize tracing: %v", err)
	} else if tracerProvider.IsEnabled() {
		debugLogger.Println("OpenTelemetry tracing initialized and enabled")
	} else {
		debugLogger.Println("OpenTelemetry tracing disabled (set OTEL_TRACES_ENABLED=true to enable)")
	}
	
	llmService := llm.NewService(apiKey, debugLogger)
	debugLogger.Println("Starting text adventure with debug logging")
	
	logger, err := logging.NewCompletionLogger()
	if err != nil {
		return ui.Model{}, nil, fmt.Errorf("failed to initialize completion logger: %w", err)
	}
	
	debugLogger.Println("Initializing MCP client...")
	mcpClient, err := mcp.NewWorldStateClient(debugMode)
	if err != nil {
		return ui.Model{}, nil, fmt.Errorf("failed to initialize MCP client: %w", err)
	}
	
	debugLogger.Println("Connecting to MCP server...")
	if err := mcpClient.Connect(ctx); err != nil {
		return ui.Model{}, nil, fmt.Errorf("failed to connect to MCP server: %w", err)
	}
	
	debugLogger.Println("Fetching initial world state from MCP server...")
	mcpWorld, err := mcpClient.GetWorldState(ctx)
	if err != nil {
		return ui.Model{}, nil, fmt.Errorf("failed to get initial world state: %w", err)
	}
	
	debugLogger.Printf("MCP world: player at %s, inventory: %v", mcpWorld.Player.Location, mcpWorld.Player.Inventory)
	
	world := mcp.MCPToGameWorldState(mcpWorld)
	
	debugLogger.Printf("Game world converted: player at %s, inventory: %v", world.Location, world.Inventory)
	
	loggers := ui.GameLoggers{
		Debug:      debugLogger,
		Completion: logger,
	}
	model := ui.NewModel(llmService, mcpClient, loggers, world)
	
	cleanup := func() {
		model.Cleanup()
		if tracerProvider != nil {
			tracerProvider.Shutdown(context.Background())
		}
	}
	
	return model, cleanup, nil
}