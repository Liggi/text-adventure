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

func BuildNPCWorldContext(npcID string, world game.WorldState, gameHistory []string) string {
	if _, exists := world.NPCs[npcID]; !exists {
		return fmt.Sprintf("ERROR: NPC %s not found", npcID)
	}
	return game.BuildWorldContext(world, gameHistory, npcID)
}

func BuildNPCWorldContextWithSenses(npcID string, world game.WorldState, sensoryEvents *sensory.SensoryEventResponse) string {
	if _, exists := world.NPCs[npcID]; !exists {
		return "ERROR: NPC not found"
	}
	
	baseContext := game.BuildWorldContext(world, []string{}, npcID)
	
	// Add sensory events that the NPC can perceive
	if sensoryEvents != nil && len(sensoryEvents.AuditoryEvents) > 0 {
		npc := world.NPCs[npcID]
		sensoryContext := "RECENT SOUNDS:\n"
		for _, event := range sensoryEvents.AuditoryEvents {
			distance := sensory.CalculateRoomDistance(npc.Location, event.Location, world.Locations)
			decayedVolume := sensory.ApplyVolumeDecay(event.Volume, distance)
			
			if decayedVolume != "" {
				if distance == 0 {
					sensoryContext += fmt.Sprintf("- %s (heard clearly)\n", event.Description)
				} else {
					sensoryContext += fmt.Sprintf("- %s (heard %s from %s)\n", event.Description, decayedVolume, event.Location)
				}
			}
		}
		sensoryContext += "\n"
		
		// Insert sensory events before the conversation history
		if strings.Contains(baseContext, "RECENT CONVERSATION:") {
			return strings.Replace(baseContext, "RECENT CONVERSATION:", sensoryContext+"RECENT CONVERSATION:", 1)
		} else {
			return baseContext + sensoryContext
		}
	}
	
	return baseContext
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