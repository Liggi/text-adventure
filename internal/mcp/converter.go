package mcp

import "textadventure/internal/game"

func MCPToGameWorldState(mcpWorld *WorldState) game.WorldState {
	gameLocations := make(map[string]game.LocationInfo)
	
	for locID, mcpLoc := range mcpWorld.Locations {
		gameLocations[locID] = game.LocationInfo{
			Title:       mcpLoc.Title,
			Description: mcpLoc.Description,
			Items:       mcpLoc.Items,
			Exits:       mcpLoc.Exits,
		}
	}
	
	gameNPCs := make(map[string]game.NPCInfo)
	for npcID, mcpNPC := range mcpWorld.NPCs {
		gameNPCs[npcID] = game.NPCInfo{
			Location:       mcpNPC.Location,
			DebugColor:     mcpNPC.DebugColor,
			Description:    mcpNPC.Description,
			Inventory:      mcpNPC.Inventory,
			RecentThoughts: mcpNPC.RecentThoughts,
			RecentActions:  mcpNPC.RecentActions,
			Personality:    mcpNPC.Personality,
			Backstory:      mcpNPC.Backstory,
			CoreMemories:   mcpNPC.CoreMemories,
		}
	}
	
	return game.WorldState{
		Location:  mcpWorld.Player.Location,
		Inventory: mcpWorld.Player.Inventory,
		MetNPCs:   mcpWorld.Player.MetNPCs,
		Locations: gameLocations,
		NPCs:      gameNPCs,
	}
}

func GameToMCPWorldState(gameWorld game.WorldState) *WorldState {
	mcpLocations := make(map[string]Location)
	
	for locID, gameLoc := range gameWorld.Locations {
		mcpLocations[locID] = Location{
			Title:       gameLoc.Title,
			Description: gameLoc.Description,
			Items:       gameLoc.Items,
			Exits:       gameLoc.Exits,
			DoorStates:  make(map[string]Door),
		}
	}
	
	return &WorldState{
		Player: Player{
			Location:  gameWorld.Location,
			Inventory: gameWorld.Inventory,
		},
		Locations: mcpLocations,
		Items:     make(map[string]Item),
	}
}