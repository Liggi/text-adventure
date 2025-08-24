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
            context.WriteString(fmt.Sprintf("NPC %s Location: %s\n", npcID, currentLoc.Name))
            
            // Show established facts about the location
            if len(currentLoc.Facts) > 0 {
                context.WriteString("Established Facts:\n")
                for _, fact := range currentLoc.Facts {
                    context.WriteString(fmt.Sprintf("- %s\n", fact))
                }
            }

            // People context first
            if world.Location == npc.Location {
                context.WriteString("Player is also here\n")
                context.WriteString(fmt.Sprintf("Player Inventory: %v\n", world.Inventory))
            }
            var otherNPCs []string
            for otherNPCID, otherNPC := range world.NPCs {
                if otherNPCID != npcID && otherNPC.Location == npc.Location {
                    otherNPCs = append(otherNPCs, otherNPCID)
                }
            }
            if len(otherNPCs) > 0 {
                context.WriteString(fmt.Sprintf("Other NPCs here: %v\n", otherNPCs))
            }

            // Navigation next
            context.WriteString(fmt.Sprintf("Available Exits: %v\n", currentLoc.Exits))

        }
	} else {
		// Player perspective
		currentLoc := world.Locations[world.Location]
		context.WriteString("Player Location: " + currentLoc.Name + "\n")
        
        // Show established facts about the location
        if len(currentLoc.Facts) > 0 {
            context.WriteString("Established Facts:\n")
            for _, fact := range currentLoc.Facts {
                context.WriteString(fmt.Sprintf("- %s\n", fact))
            }
        }
        // People context first
        var npcsHere []string
        for npcID, npc := range world.NPCs {
            if npc.Location == world.Location {
                met := false
                for _, metNPC := range world.MetNPCs {
                    if metNPC == npcID {
                        met = true
                        break
                    }
                }
                if met {
                    npcsHere = append(npcsHere, npcID)
                } else {
                    description := npc.Description
                    if description == "" {
                        description = "someone"
                    }
                    npcsHere = append(npcsHere, description)
                }
            }
        }
        if len(npcsHere) > 0 {
            context.WriteString(fmt.Sprintf("People here: %v\n", npcsHere))
        }
        // Navigation next
        context.WriteString(fmt.Sprintf("Available Exits: %v\n", currentLoc.Exits))
        // Inventory and items last
        context.WriteString(fmt.Sprintf("Player Inventory: %v\n", world.Inventory))
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
