package actors

import (
    "fmt"
    "strings"
)

func buildThoughtsPrompt(npcID string, recentThoughts []string, recentActions []string, personality string, backstory string, coreMemories []string) string {
	memoryContext := ""
	if len(recentThoughts) > 0 || len(recentActions) > 0 {
		memoryContext = "\n\nYour recent memory:"
		if len(recentThoughts) > 0 {
			memoryContext += fmt.Sprintf("\nPrevious thoughts: %v", recentThoughts)
		}
		if len(recentActions) > 0 {
			memoryContext += fmt.Sprintf("\nPrevious actions: %v", recentActions)
		}
		memoryContext += "\nDon't repeat the same thoughts unless something has changed. Build on your previous thinking."
	}

	personalityContext := ""
	if personality != "" {
		personalityContext = fmt.Sprintf("\n- Personality: %s", personality)
	}
	
	backstoryContext := ""
	if backstory != "" {
		backstoryContext = fmt.Sprintf("\n\nWho you are: %s", backstory)
	}
	
	coreMemoryContext := ""
	if len(coreMemories) > 0 {
		coreMemoryContext = "\n\nCore memories that shape your thinking:"
		for _, memory := range coreMemories {
			coreMemoryContext += fmt.Sprintf("\n- %s", memory)
		}
	}

	return fmt.Sprintf(`You are %s, an NPC in a text adventure game. Generate realistic internal thoughts - the kind that actually run through someone's head.

Your character:
- Name: %s%s
- You think like a real person - brief, practical thoughts, not flowery descriptions
- React to immediate surroundings and events%s%s

Think like an actual human mind:
- Use simple, direct thoughts: "Should I check that out?" not "The crystal orb hums faintly—why now"
- Be practical, not poetic
- Notice connections naturally: if you hear footsteps then see someone, you'd casually connect them
- Think ahead: "Maybe I should..." or "I wonder if..."
- Base thoughts on what you can see, hear, remember, and what's been happening around you
- Let your personality and background influence your reactions%s

Return only realistic internal thoughts, nothing else. Keep it to one line.`, npcID, npcID, personalityContext, backstoryContext, coreMemoryContext, memoryContext)
}

// buildThoughtsPromptXML produces a clearer, sectioned system prompt for NPC thinking.
// It uses simple XML-like tags to make parsing and emphasis reliable.
func buildThoughtsPromptXML(npcID string, recentThoughts []string, recentActions []string, personality string, backstory string, coreMemories []string) string {
    b := &strings.Builder{}
    fmt.Fprintf(b, `You are %s. Generate a single internal thought based on your current situation.`, npcID)
    b.WriteString("\n\n<character>\n")
    fmt.Fprintf(b, "- name: %s\n", npcID)
    if strings.TrimSpace(personality) != "" {
        fmt.Fprintf(b, "- personality: %s\n", personality)
    }
    if strings.TrimSpace(backstory) != "" {
        fmt.Fprintf(b, "- backstory: %s\n", backstory)
    }
    if len(coreMemories) > 0 {
        b.WriteString("- core_memories:\n")
        for _, m := range coreMemories {
            fmt.Fprintf(b, "  - %s\n", m)
        }
    }
    b.WriteString("</character>\n\n")

    b.WriteString("<recent_memory>\n")
    if len(recentThoughts) > 0 {
        b.WriteString("- thoughts:\n")
        for _, t := range recentThoughts {
            fmt.Fprintf(b, "  - %s\n", t)
        }
    }
    if len(recentActions) > 0 {
        b.WriteString("- actions:\n")
        for _, a := range recentActions {
            fmt.Fprintf(b, "  - %s\n", a)
        }
    }
    b.WriteString("</recent_memory>\n\n")

    b.WriteString(`<style>
- one line only
- present tense; natural and practical
- base only on world_context and perceived_events
- no quotes; no role labels; no narration
- avoid repeating identical prior thoughts; build on change
- it's fine to be uncertain or to simply observe; don't force a plan
</style>`)        
    return b.String()
}

// buildNPCThoughtsUserXML wraps the dynamic context for the NPC think step.
func buildNPCThoughtsUserXML(worldContext string, perceivedLines []string, situation string) string {
    b := &strings.Builder{}
    b.WriteString("<world_context>\n")
    b.WriteString(strings.TrimSpace(worldContext))
    b.WriteString("\n</world_context>\n\n")
    if strings.TrimSpace(situation) != "" {
        b.WriteString("<situation>\n")
        b.WriteString(strings.TrimSpace(situation))
        b.WriteString("\n</situation>\n\n")
    }
    b.WriteString("<perceived_events>\n")
    for _, ev := range perceivedLines {
        fmt.Fprintf(b, "- %s\n", strings.TrimSpace(ev))
    }
    b.WriteString("</perceived_events>")
    return b.String()
}

// buildNPCSituationUser builds the user prompt for situation summarization
func buildNPCSituationUser(worldContext string, perceivedLines []string) string {
    b := &strings.Builder{}
    b.WriteString("<world_context>\n")
    b.WriteString(strings.TrimSpace(worldContext))
    b.WriteString("\n</world_context>\n\n")
    b.WriteString("<perceived_events>\n")
    for _, ev := range perceivedLines {
        fmt.Fprintf(b, "- %s\n", strings.TrimSpace(ev))
    }
    b.WriteString("</perceived_events>")
    return b.String()
}

func xmlLineIf(tag, val string) string {
    if strings.TrimSpace(val) == "" {
        return ""
    }
    return fmt.Sprintf("<%s>%s</%s>", tag, val, tag)
}

func buildActionPrompt(npcID string, npcThoughts string, recentActions []string, personality string, backstory string) string {
	memoryContext := ""
	if len(recentActions) > 0 {
		memoryContext = fmt.Sprintf("\n\nYour recent actions: %v\nDon't repeat the same action unless something has changed.", recentActions)
	}

	personalityContext := ""
	if personality != "" {
		personalityContext = fmt.Sprintf("- Personality: %s\n", personality)
	}
	
	backstoryContext := ""
	if backstory != "" {
		backstoryContext = fmt.Sprintf("- Background: %s\n", backstory)
	}

	return fmt.Sprintf(`You are %s. React realistically to your current situation — you don't have to "pick an action" every turn.

Your character:
- Name: %s
%s%s- You act naturally based on what you've noticed and what you're thinking
- You can move between rooms, talk to people, interact with objects, or simply pause to observe or think
- Only act if it makes sense right now; it's valid to call out, look around, or do nothing

Your current thoughts: "%s"%s

Based on your thoughts and the world state, what do you want to do? You can:
- Move to a different room (e.g., "go to kitchen") 
- Say something (e.g., "say Hello there!")
- Pick up an item (e.g., "take key")
- Look around or examine something (e.g., "look around", "examine desk")
- Call out (e.g., "say Is someone there?")
- Do nothing (return empty string)

Return only a brief action statement, or an empty string if you don't want to act.`, npcID, npcID, personalityContext, backstoryContext, npcThoughts, memoryContext)
}
