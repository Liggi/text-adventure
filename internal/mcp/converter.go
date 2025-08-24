package mcp

import "textadventure/internal/game"

func MCPToGameWorldState(mcpWorld *WorldState) game.WorldState {
	gameLocations := make(map[string]game.LocationInfo)
	
	for locID, mcpLoc := range mcpWorld.Locations {
		gameLocations[locID] = game.LocationInfo{
			Name:  mcpLoc.Name,
			Facts: mcpLoc.Facts,
			Exits: mcpLoc.Exits,
		}
	}
	
	gameNPCs := make(map[string]game.NPCInfo)
	for npcID, mcpNPC := range mcpWorld.NPCs {
		gameNPCs[npcID] = game.NPCInfo{
			Location:       mcpNPC.Location,
			DebugColor:     mcpNPC.DebugColor,
			Description:    mcpNPC.Name,
			Inventory:      mcpNPC.Inventory,
			RecentThoughts: mcpNPC.RecentThoughts,
			RecentActions:  mcpNPC.RecentActions,
			Personality:    mcpNPC.Personality,
			Backstory:      mcpNPC.Backstory,
			Memories:       mcpNPC.Memories,
			Facts:          mcpNPC.Facts,
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
			Name:       gameLoc.Name,
			Facts:      gameLoc.Facts,
			Exits:      gameLoc.Exits,
			DoorStates: make(map[string]Door),
		}
	}
	
	mcpNPCs := make(map[string]NPC)
	for npcID, gameNPC := range gameWorld.NPCs {
		mcpNPCs[npcID] = NPC{
			Name:           gameNPC.Description,
			Location:       gameNPC.Location,
			DebugColor:     gameNPC.DebugColor,
			Facts:          gameNPC.Facts,
			Inventory:      gameNPC.Inventory,
			RecentThoughts: gameNPC.RecentThoughts,
			RecentActions:  gameNPC.RecentActions,
			Personality:    gameNPC.Personality,
			Backstory:      gameNPC.Backstory,
			Memories:       gameNPC.Memories,
		}
	}
	
	return &WorldState{
		Player: Player{
			Location:  gameWorld.Location,
			Inventory: gameWorld.Inventory,
			MetNPCs:   gameWorld.MetNPCs,
		},
		Locations: mcpLocations,
		Items:     make(map[string]Item),
		NPCs:      mcpNPCs,
	}
}