package actors

import (
    "context"
    "fmt"
    "log"
    "strings"

	tea "github.com/charmbracelet/bubbletea"

	"textadventure/internal/game"
	"textadventure/internal/game/sensory"
	"textadventure/internal/llm"
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
		sensoryContext := "SOUNDS THAT JUST OCCURRED:\n"
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
func GenerateNPCThoughts(ctx context.Context, llmService *llm.Service, npcID string, world game.WorldState, gameHistory []string, debug bool, sensoryEvents *sensory.SensoryEventResponse) tea.Cmd {
    return func() tea.Msg {
        worldContext := BuildNPCWorldContextWithSenses(npcID, world, sensoryEvents)
		
		var recentThoughts, recentActions []string
		var personality, backstory string
		var coreMemories []string
		if npc, exists := world.NPCs[npcID]; exists {
			recentThoughts = npc.RecentThoughts
			recentActions = npc.RecentActions
			personality = npc.Personality
			backstory = npc.Backstory
			coreMemories = npc.CoreMemories
		}
		
		req := llm.TextCompletionRequest{
			SystemPrompt: buildThoughtsPrompt(npcID, recentThoughts, recentActions, personality, backstory, coreMemories),
			UserPrompt:   worldContext,
			MaxTokens:    150,
		}

        ctx = llm.WithOperationType(ctx, "npc.think")
        ctx = llm.WithGameContext(ctx, map[string]interface{}{
            "npc_id":   npcID,
            "location": world.NPCs[npcID].Location,
        })
        thoughts, err := llmService.CompleteText(ctx, req)
		if err != nil {
			return NPCThoughtsMsg{
				NPCID:    npcID,
				Thoughts: "",
				Debug:    debug,
			}
		}

		thoughts = strings.TrimSpace(thoughts)

		return NPCThoughtsMsg{
			NPCID:    npcID,
			Thoughts: thoughts,
			Debug:    debug,
		}
	}
}

// GenerateNPCAction generates an action for an NPC based on their thoughts and world state
func GenerateNPCAction(ctx context.Context, llmService *llm.Service, npcID string, npcThoughts string, world game.WorldState, sensoryEvents *sensory.SensoryEventResponse, debug bool) (string, error) {
	if npcThoughts == "" {
		return "", nil
	}

	worldContext := BuildNPCWorldContextWithSenses(npcID, world, sensoryEvents)
	
	var recentActions []string
	var personality, backstory string
	if npc, exists := world.NPCs[npcID]; exists {
		recentActions = npc.RecentActions
		personality = npc.Personality
		backstory = npc.Backstory
	}
	
	req := llm.TextCompletionRequest{
		SystemPrompt: buildActionPrompt(npcID, npcThoughts, recentActions, personality, backstory),
		UserPrompt:   worldContext,
		MaxTokens:    100,
	}

    ctx = llm.WithOperationType(ctx, "npc.act")
    ctx = llm.WithGameContext(ctx, map[string]interface{}{
        "npc_id":      npcID,
        "location":    world.NPCs[npcID].Location,
        "has_thoughts": len(npcThoughts) > 0,
    })
    action, err := llmService.CompleteText(ctx, req)
	if err != nil {
		return "", err
	}

	action = strings.TrimSpace(action)

	return action, nil
}

// GenerateNPCTurn creates a tea.Cmd that handles a complete NPC turn (thoughts + action)
func GenerateNPCTurn(ctx context.Context, llmService *llm.Service, npcID string, world game.WorldState, gameHistory []string, debug bool, sensoryEvents *sensory.SensoryEventResponse) tea.Cmd {
    return func() tea.Msg {
        thoughts := ""
		if debug {
			worldContext := BuildNPCWorldContextWithSenses(npcID, world, sensoryEvents)
			log.Printf("=== NPC TURN START ===")
			log.Printf("NPC: %s", npcID)
			log.Printf("World context length: %d chars", len(worldContext))
		}

        thoughtsMsg := GenerateNPCThoughts(ctx, llmService, npcID, world, gameHistory, debug, sensoryEvents)()
        if msg, ok := thoughtsMsg.(NPCThoughtsMsg); ok {
            thoughts = msg.Thoughts
        }

        action, err := GenerateNPCAction(ctx, llmService, npcID, thoughts, world, sensoryEvents, debug)
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
