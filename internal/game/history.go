package game

import (
	"fmt"
	"strings"
)

type History struct {
	exchanges []string
	maxSize   int
}

func NewHistory(maxSize int) *History {
	return &History{
		exchanges: make([]string, 0, maxSize),
		maxSize:   maxSize,
	}
}

func (h *History) AddPlayerAction(input string) {
	h.add("Player: " + input)
}

func (h *History) AddNarratorResponse(response string) {
	h.add("Narrator: " + response)
}

func (h *History) AddNPCAction(npcID, action string) {
	h.add(fmt.Sprintf("%s: %s", npcID, action))
}

func (h *History) AddError(err error) {
	h.add("Error: " + err.Error())
}

func (h *History) add(entry string) {
	h.exchanges = append(h.exchanges, entry)
	
	if len(h.exchanges) > h.maxSize {
		h.exchanges = h.exchanges[len(h.exchanges)-h.maxSize:]
	}
}

func (h *History) GetEntries() []string {
	result := make([]string, len(h.exchanges))
	copy(result, h.exchanges)
	return result
}


// BuildWorldContext creates a comprehensive formatted context string for LLMs.
// It handles both player and NPC perspectives, including co-location detection,
// world state, and conversation history.
func BuildWorldContext(world WorldState, gameHistory []string, actingNPCID ...string) string {
	var context strings.Builder
	
	context.WriteString("WORLD STATE:\n")
	
	if len(actingNPCID) > 0 && actingNPCID[0] != "" {
		// NPC perspective
		npcID := actingNPCID[0]
		if npc, exists := world.NPCs[npcID]; exists {
			currentLoc := world.Locations[npc.Location]
			context.WriteString(fmt.Sprintf("NPC %s Location: %s (%s)\n", npcID, currentLoc.Title, npc.Location))
			context.WriteString(currentLoc.Description + "\n")
			context.WriteString(fmt.Sprintf("Available Items Here: %v\n", currentLoc.Items))
			context.WriteString(fmt.Sprintf("Available Exits: %v\n", currentLoc.Exits))
			
			// Show co-location with player
			if world.Location == npc.Location {
				context.WriteString("Player is also here\n")
				context.WriteString(fmt.Sprintf("Player Inventory: %v\n", world.Inventory))
			}
			
			// Show other NPCs at this location
			var otherNPCs []string
			for otherNPCID, otherNPC := range world.NPCs {
				if otherNPCID != npcID && otherNPC.Location == npc.Location {
					otherNPCs = append(otherNPCs, otherNPCID)
				}
			}
			if len(otherNPCs) > 0 {
				context.WriteString(fmt.Sprintf("Other NPCs here: %v\n", otherNPCs))
			}
		}
	} else {
		// Player perspective
		currentLoc := world.Locations[world.Location]
		context.WriteString("Player Location: " + currentLoc.Title + " (" + world.Location + ")\n")
		context.WriteString(currentLoc.Description + "\n")
		context.WriteString(fmt.Sprintf("Player Inventory: %v\n", world.Inventory))
		context.WriteString(fmt.Sprintf("Available Items Here: %v\n", currentLoc.Items))
		context.WriteString(fmt.Sprintf("Available Exits: %v\n", currentLoc.Exits))
		
		// Show NPCs at player's location
		var npcsHere []string
		for npcID, npc := range world.NPCs {
			if npc.Location == world.Location {
				npcsHere = append(npcsHere, npcID)
			}
		}
		if len(npcsHere) > 0 {
			context.WriteString(fmt.Sprintf("NPCs here: %v\n", npcsHere))
		}
	}
	
	
	if len(gameHistory) > 0 {
		context.WriteString("RECENT CONVERSATION:\n")
		for _, exchange := range gameHistory {
			context.WriteString(exchange + "\n")
		}
		context.WriteString("\n")
	}
	
	return context.String()
}