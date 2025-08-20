package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type WorldStateClient struct {
	client  *mcp.Client
	session *mcp.ClientSession
	debug   bool
}

type WorldState struct {
	Player    Player               `json:"player"`
	Locations map[string]Location  `json:"locations"`
	Items     map[string]Item      `json:"items"`
	NPCs      map[string]NPC       `json:"npcs"`
}

type Player struct {
	Location  string   `json:"location"`
	Inventory []string `json:"inventory"`
}

type Location struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Items       []string          `json:"items"`
	Exits       map[string]string `json:"exits"`
	DoorStates  map[string]Door   `json:"door_states"`
}

type Door struct {
	Locked      bool   `json:"locked"`
	Description string `json:"description"`
}

type Item struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	CanUnlock   []string `json:"can_unlock"`
}

type NPC struct {
	Location   string   `json:"location"`
	DebugColor string   `json:"debug_color"`
	Inventory  []string `json:"inventory"`
}

func NewWorldStateClient(debug bool) (*WorldStateClient, error) {
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "text-adventure-client",
		Version: "v1.0.0",
	}, nil)

	return &WorldStateClient{
		client: client,
		debug:  debug,
	}, nil
}

func (w *WorldStateClient) Connect(ctx context.Context) error {
	cmd := exec.Command("uv", "run", "python", "world_state.py")
	cmd.Dir = "services/worldstate"
	
	transport := mcp.NewCommandTransport(cmd)

	session, err := w.client.Connect(ctx, transport)
	if err != nil {
		return fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	w.session = session

	if w.debug {
		log.Println("Connected to MCP world state server")
	}

	return nil
}

func (w *WorldStateClient) Close() error {
	if w.session != nil {
		w.session.Close()
	}
	return nil
}

func (w *WorldStateClient) GetWorldState(ctx context.Context) (*WorldState, error) {
	params := &mcp.CallToolParams{
		Name:      "get_world_state",
		Arguments: nil,
	}

	result, err := w.session.CallTool(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get world state: %w", err)
	}

	if result.IsError {
		errorMsg := result.Content[0].(*mcp.TextContent).Text
		return nil, fmt.Errorf(errorMsg)
	}

	var worldState WorldState
	if err := json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &worldState); err != nil {
		return nil, fmt.Errorf("failed to parse world state: %w", err)
	}

	if w.debug {
		log.Printf("Retrieved world state: player at %s", worldState.Player.Location)
	}

	return &worldState, nil
}

func (w *WorldStateClient) MovePlayer(ctx context.Context, location string) (string, error) {
	params := &mcp.CallToolParams{
		Name:      "move_player",
		Arguments: map[string]interface{}{"location": location},
	}

	result, err := w.session.CallTool(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to move player: %w", err)
	}

	response := result.Content[0].(*mcp.TextContent).Text
	if result.IsError {
		return response, fmt.Errorf(response)
	}
	if w.debug {
		log.Printf("Move player result: %s", response)
	}

	return response, nil
}

func (w *WorldStateClient) AddToInventory(ctx context.Context, item string) (string, error) {
	params := &mcp.CallToolParams{
		Name:      "add_to_inventory",
		Arguments: map[string]interface{}{"item": item},
	}

	result, err := w.session.CallTool(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to add to inventory: %w", err)
	}

	response := result.Content[0].(*mcp.TextContent).Text
	if result.IsError {
		return response, fmt.Errorf(response)
	}
	if w.debug {
		log.Printf("Add to inventory result: %s", response)
	}

	return response, nil
}

func (w *WorldStateClient) RemoveFromInventory(ctx context.Context, item string) (string, error) {
	params := &mcp.CallToolParams{
		Name:      "remove_from_inventory",
		Arguments: map[string]interface{}{"item": item},
	}

	result, err := w.session.CallTool(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to remove from inventory: %w", err)
	}

	response := result.Content[0].(*mcp.TextContent).Text
	if result.IsError {
		return response, fmt.Errorf(response)
	}
	if w.debug {
		log.Printf("Remove from inventory result: %s", response)
	}

	return response, nil
}

func (w *WorldStateClient) UnlockDoor(ctx context.Context, location, direction, keyItem string) (string, error) {
	params := &mcp.CallToolParams{
		Name: "unlock_door",
		Arguments: map[string]interface{}{
			"location":  location,
			"direction": direction,
			"key_item":  keyItem,
		},
	}

	result, err := w.session.CallTool(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to unlock door: %w", err)
	}

	response := result.Content[0].(*mcp.TextContent).Text
	if result.IsError {
		return response, fmt.Errorf(response)
	}
	if w.debug {
		log.Printf("Unlock door result: %s", response)
	}

	return response, nil
}

func (w *WorldStateClient) TransferItem(ctx context.Context, item, fromLocation, toLocation string) (string, error) {
	params := &mcp.CallToolParams{
		Name: "transfer_item",
		Arguments: map[string]interface{}{
			"item":          item,
			"from_location": fromLocation,
			"to_location":   toLocation,
		},
	}

	result, err := w.session.CallTool(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to transfer item: %w", err)
	}

	response := result.Content[0].(*mcp.TextContent).Text
	if result.IsError {
		return response, fmt.Errorf(response)
	}
	if w.debug {
		log.Printf("Transfer item result: %s", response)
	}

	return response, nil
}

func (w *WorldStateClient) ListTools(ctx context.Context) (string, error) {
	params := &mcp.ListToolsParams{}
	
	result, err := w.session.ListTools(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to list tools: %w", err)
	}
	
	toolDescriptions := make([]string, 0, len(result.Tools))
	for _, tool := range result.Tools {
		description := fmt.Sprintf("- %s: %s", tool.Name, tool.Description)
		if tool.InputSchema != nil {
			schemaJson, _ := json.Marshal(tool.InputSchema)
			description += fmt.Sprintf(" (Schema: %s)", string(schemaJson))
		}
		toolDescriptions = append(toolDescriptions, description)
	}
	
	return strings.Join(toolDescriptions, "\n"), nil
}