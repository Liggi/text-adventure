package actors

import (
    "context"
    "fmt"
    "log"
    "strings"

    tea "github.com/charmbracelet/bubbletea"

    "textadventure/internal/game"
    "textadventure/internal/game/perception"
    "textadventure/internal/llm"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
)

func BuildNPCWorldContext(npcID string, world game.WorldState, gameHistory []string) string {
	if _, exists := world.NPCs[npcID]; !exists {
		return fmt.Sprintf("ERROR: NPC %s not found", npcID)
	}
	return game.BuildWorldContext(world, gameHistory, npcID)
}

func BuildNPCWorldContextWithPerceptions(npcID string, world game.WorldState, perceivedLines []string) string {
    if _, exists := world.NPCs[npcID]; !exists {
        return "ERROR: NPC not found"
    }

    baseContext := game.BuildWorldContext(world, []string{}, npcID)
    if len(perceivedLines) == 0 {
        return baseContext
    }

    b := &strings.Builder{}
    b.WriteString("PERCEIVED EVENTS:\n")
    for _, line := range perceivedLines {
        fmt.Fprintf(b, "- %s\n", strings.TrimSpace(line))
    }
    b.WriteString("\n")
    if strings.Contains(baseContext, "RECENT CONVERSATION:") {
        return strings.Replace(baseContext, "RECENT CONVERSATION:", b.String()+"RECENT CONVERSATION:", 1)
    }
    return baseContext + b.String()
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
    Debug         bool
}

// GenerateNPCThoughts creates a tea.Cmd that generates thoughts for an NPC
func GenerateNPCThoughts(ctx context.Context, llmService *llm.Service, npcID string, world game.WorldState, gameHistory []string, debug bool, perceivedLines []string, situation string) tea.Cmd {
    return func() tea.Msg {
        worldContext := game.BuildWorldContext(world, []string{}, npcID)
		
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
            SystemPrompt: buildThoughtsPromptXML(npcID, recentThoughts, recentActions, personality, backstory, coreMemories),
            UserPrompt:   buildNPCThoughtsUserXML(worldContext, perceivedLines, situation),
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
func GenerateNPCAction(ctx context.Context, llmService *llm.Service, npcID string, npcThoughts string, world game.WorldState, perceivedLines []string, debug bool) (string, error) {
    if npcThoughts == "" {
        return "", nil
    }

    worldContext := BuildNPCWorldContextWithPerceptions(npcID, world, perceivedLines)
	
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
func GenerateNPCTurn(ctx context.Context, llmService *llm.Service, npcID string, world game.WorldState, gameHistory []string, debug bool, worldEventLines []string) tea.Cmd {
    return func() tea.Msg {
        thoughts := ""
        situation := ""
        if debug {
            worldContext := game.BuildWorldContext(world, []string{}, npcID)
            log.Printf("=== NPC TURN START ===")
            log.Printf("NPC: %s", npcID)
            log.Printf("World context length: %d chars", len(worldContext))
        }

        // LLM-driven perception per NPC
        tracer := otel.Tracer("perception")
        pctx, pspan := tracer.Start(ctx, "perception.llm")
        perceivedLines, perr := perception.GeneratePerceivedEventsForNPC(pctx, llmService, npcID, world, worldEventLines)
        if perr != nil && debug {
            log.Printf("Perception error for %s: %v", npcID, perr)
        }
        pspan.SetAttributes(
            attribute.String("npc.id", npcID),
            attribute.Int("events.input_count", len(worldEventLines)),
            attribute.Int("events.perceived_count", len(perceivedLines)),
        )
        pspan.End()

        // Lightweight situation narration to bridge "just happened" and "now"
        if true { // always try to produce a minimal situation summary
            sctx, sspan := otel.Tracer("perception").Start(ctx, "perception.situation")
            s := buildNPCSituationUser(game.BuildWorldContext(world, []string{}, npcID), perceivedLines)
            req := llm.TextCompletionRequest{
                SystemPrompt: `Summarize the immediate situation in 1-2 short sentences in present tense.
Use only the provided world_context and perceived_events.
Be concrete and neutral. No invention beyond those details.`,
                UserPrompt:   s,
                MaxTokens:    60,
                Model:        "gpt-5-mini",
            }
            sctx = llm.WithOperationType(sctx, "npc.situation")
            out, serr := llmService.CompleteText(sctx, req)
            if serr == nil {
                situation = strings.TrimSpace(out)
            } else if debug {
                log.Printf("Situation summary failed for %s: %v", npcID, serr)
            }
            sspan.End()
        }

        thoughtsMsg := GenerateNPCThoughts(ctx, llmService, npcID, world, gameHistory, debug, perceivedLines, situation)()
        if msg, ok := thoughtsMsg.(NPCThoughtsMsg); ok {
            thoughts = msg.Thoughts
        }

        action, err := GenerateNPCAction(ctx, llmService, npcID, thoughts, world, perceivedLines, debug)
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
            Debug:         debug,
        }
    }
}
