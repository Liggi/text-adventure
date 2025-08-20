package actors

import (
	"context"
	"fmt"
	"log"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sashabaranov/go-openai"

	"textadventure/internal/game"
	"textadventure/internal/game/sensory"
)

// BuildNPCWorldContext creates world context for an NPC without sensory information
func BuildNPCWorldContext(npcID string, world game.WorldState, gameHistory []string) string {
	npc, exists := world.NPCs[npcID]
	if !exists {
		// Fallback - this would need buildWorldContext from commands.go, but we'll handle that later
		return fmt.Sprintf("ERROR: NPC %s not found", npcID)
	}
	
	currentLoc := world.Locations[npc.Location]
	context := "WORLD STATE:\n"
	context += "Current Location: " + currentLoc.Title + " (" + npc.Location + ")\n"
	context += currentLoc.Description + "\n"
	
	var peopleHere []string
	if world.Location == npc.Location {
		peopleHere = append(peopleHere, "player")
	}
	for otherNPCID, otherNPC := range world.NPCs {
		if otherNPCID != npcID && otherNPC.Location == npc.Location {
			peopleHere = append(peopleHere, otherNPCID)
		}
	}
	if len(peopleHere) > 0 {
		context += "\nPeople here: " + fmt.Sprintf("%v", peopleHere) + "\n"
	}
	context += "\n"
	
	context += "Available Items Here: " + fmt.Sprintf("%v", currentLoc.Items) + "\n"
	context += "Available Exits: " + fmt.Sprintf("%v", currentLoc.Exits) + "\n"
	
	if world.Location == npc.Location {
		context += "Player Inventory: " + fmt.Sprintf("%v", world.Inventory) + "\n"
	}
	context += "\n"

	if len(gameHistory) > 0 {
		context += "RECENT CONVERSATION:\n"
		for _, exchange := range gameHistory {
			context += exchange + "\n"
		}
		context += "\n"
	}

	return context
}

// BuildNPCWorldContextWithSenses creates world context for an NPC including sensory events
func BuildNPCWorldContextWithSenses(npcID string, world game.WorldState, sensoryEvents *sensory.SensoryEventResponse) string {
	npc, exists := world.NPCs[npcID]
	if !exists {
		return "ERROR: NPC not found"
	}
	
	currentLoc := world.Locations[npc.Location]
	context := "WORLD STATE:\n"
	context += "Current Location: " + currentLoc.Title + " (" + npc.Location + ")\n"
	context += currentLoc.Description + "\n"
	
	var peopleHere []string
	if world.Location == npc.Location {
		peopleHere = append(peopleHere, "player")
	}
	for otherNPCID, otherNPC := range world.NPCs {
		if otherNPCID != npcID && otherNPC.Location == npc.Location {
			peopleHere = append(peopleHere, otherNPCID)
		}
	}
	if len(peopleHere) > 0 {
		context += "\nPeople here: " + fmt.Sprintf("%v", peopleHere) + "\n"
	}
	context += "\n"
	
	context += "Available Items Here: " + fmt.Sprintf("%v", currentLoc.Items) + "\n"
	context += "Available Exits: " + fmt.Sprintf("%v", currentLoc.Exits) + "\n"
	
	if world.Location == npc.Location {
		context += "Player Inventory: " + fmt.Sprintf("%v", world.Inventory) + "\n"
	}
	context += "\n"

	// Add sensory events that the NPC can perceive
	if sensoryEvents != nil && len(sensoryEvents.AuditoryEvents) > 0 {
		context += "RECENT SOUNDS:\n"
		for _, event := range sensoryEvents.AuditoryEvents {
			distance := sensory.CalculateRoomDistance(npc.Location, event.Location, world.Locations)
			decayedVolume := sensory.ApplyVolumeDecay(event.Volume, distance)
			
			if decayedVolume != "" {
				if distance == 0 {
					context += fmt.Sprintf("- %s (heard clearly)\n", event.Description)
				} else {
					context += fmt.Sprintf("- %s (heard %s from %s)\n", event.Description, decayedVolume, event.Location)
				}
			}
		}
		context += "\n"
	}

	return context
}

// NPCThoughtsMsg represents the result of NPC thought generation
type NPCThoughtsMsg struct {
	NPCID    string
	Thoughts string
	Debug    bool
}

// NPCActionMsg represents the result of NPC action generation
type NPCActionMsg struct {
	NPCID         string
	Thoughts      string
	Action        string
	SensoryEvents *sensory.SensoryEventResponse
	Debug         bool
}

// GenerateNPCThoughts creates a tea.Cmd that generates thoughts for an NPC
func GenerateNPCThoughts(client *openai.Client, npcID string, world game.WorldState, gameHistory []string, debug bool, sensoryEvents *sensory.SensoryEventResponse) tea.Cmd {
	return func() tea.Msg {
		if debug {
			worldContext := BuildNPCWorldContextWithSenses(npcID, world, sensoryEvents)
			
			if debug {
				log.Printf("=== NPC THOUGHTS GENERATION START ===")
				log.Printf("NPC: %s", npcID)
				log.Printf("World context length: %d chars", len(worldContext))
			}
		}

		systemPrompt := fmt.Sprintf(`You are %s, an NPC in a text adventure game. You need to generate your internal thoughts based on the current world state and recent events.

Your character:
- Name: %s  
- You are curious, intelligent, and responsive to your environment
- You react to sounds, people entering/leaving, and changes in your surroundings
- Keep thoughts concise and in-character

Generate your internal thoughts based on what you observe, hear, or experience. This is your private mental state - no one else can hear these thoughts.

Return only your thoughts, nothing else. Keep it to one line.`, npcID, npcID)

		worldContext := BuildNPCWorldContextWithSenses(npcID, world, sensoryEvents)
		
		req := openai.ChatCompletionRequest{
			Model: "gpt-5-2025-08-07",
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemPrompt,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: worldContext,
				},
			},
			MaxCompletionTokens: 150,
			ReasoningEffort:     "minimal",
		}

		resp, err := client.CreateChatCompletion(context.Background(), req)
		if err != nil {
			if debug {
				log.Printf("NPC thoughts generation error for %s: %v", npcID, err)
			}
			return NPCThoughtsMsg{
				NPCID:    npcID,
				Thoughts: "",
				Debug:    debug,
			}
		}

		thoughts := ""
		if len(resp.Choices) > 0 {
			thoughts = strings.TrimSpace(resp.Choices[0].Message.Content)
		}

		if debug {
			log.Printf("Generated thoughts for %s: %q", npcID, thoughts)
			log.Printf("=== NPC THOUGHTS GENERATION END ===")
		}

		return NPCThoughtsMsg{
			NPCID:    npcID,
			Thoughts: thoughts,
			Debug:    debug,
		}
	}
}

// GenerateNPCAction generates an action for an NPC based on their thoughts and world state
func GenerateNPCAction(client *openai.Client, npcID string, npcThoughts string, world game.WorldState, sensoryEvents *sensory.SensoryEventResponse, debug bool) (string, error) {
	if npcThoughts == "" {
		return "", nil
	}

	worldContext := BuildNPCWorldContextWithSenses(npcID, world, sensoryEvents)
	
	systemPrompt := fmt.Sprintf(`You are %s, an NPC in a text adventure game. Based on your thoughts and the current situation, decide what action to take.

Your character:
- Name: %s
- You are curious, intelligent, and responsive to your environment
- You can move between rooms, pick up items, talk to people, or interact with objects
- You should react naturally to sounds, people, and changes in your environment

Your current thoughts: "%s"

Based on your thoughts and the world state, what do you want to do? You can:
- Move to a different room (e.g., "go to kitchen") 
- Say something (e.g., "say Hello there!")
- Pick up an item (e.g., "take key")
- Look around or examine something
- Do nothing (return empty string)

Return only a brief action statement, or an empty string if you don't want to act.`, npcID, npcID, npcThoughts)

	req := openai.ChatCompletionRequest{
		Model: "gpt-5-2025-08-07",
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: worldContext,
			},
		},
		MaxCompletionTokens: 100,
		ReasoningEffort:     "minimal",
	}

	resp, err := client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		if debug {
			log.Printf("NPC action generation error for %s: %v", npcID, err)
		}
		return "", err
	}

	action := ""
	if len(resp.Choices) > 0 {
		action = strings.TrimSpace(resp.Choices[0].Message.Content)
	}

	if debug {
		log.Printf("Generated action for %s: %q", npcID, action)
	}

	return action, nil
}

// GenerateNPCTurn creates a tea.Cmd that handles a complete NPC turn (thoughts + action)
func GenerateNPCTurn(client *openai.Client, npcID string, world game.WorldState, gameHistory []string, debug bool, sensoryEvents *sensory.SensoryEventResponse) tea.Cmd {
	return func() tea.Msg {
		thoughts := ""
		if debug {
			worldContext := BuildNPCWorldContextWithSenses(npcID, world, sensoryEvents)
			log.Printf("=== NPC TURN START ===")
			log.Printf("NPC: %s", npcID)
			log.Printf("World context length: %d chars", len(worldContext))
		}

		thoughtsMsg := GenerateNPCThoughts(client, npcID, world, gameHistory, debug, sensoryEvents)()
		if msg, ok := thoughtsMsg.(NPCThoughtsMsg); ok {
			thoughts = msg.Thoughts
		}

		action, err := GenerateNPCAction(client, npcID, thoughts, world, sensoryEvents, debug)
		if err != nil {
			if debug {
				log.Printf("Error generating action for %s: %v", npcID, err)
			}
			action = ""
		}

		if debug {
			log.Printf("NPC %s turn complete - thoughts: %q, action: %q", npcID, thoughts, action)
			log.Printf("=== NPC TURN END ===")
		}

		return NPCActionMsg{
			NPCID:         npcID,
			Thoughts:      thoughts,
			Action:        action,
			SensoryEvents: sensoryEvents,
			Debug:         debug,
		}
	}
}