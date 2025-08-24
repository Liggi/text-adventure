package narration

import (
    "fmt"
    "strings"
)

// buildNPCNarrationPrompt builds a system prompt for NPC-perspective narration.
func BuildNPCNarrationPrompt(npcID string, actionContext string, mutationResults []string, worldEventLines []string) string {
    var actionAndMutationContext string
    if strings.TrimSpace(actionContext) != "" {
        actionAndMutationContext = fmt.Sprintf("\n\nACTION THAT JUST OCCURRED:\n%s", actionContext)
        if len(mutationResults) > 0 {
            actionAndMutationContext += "\n\nWORLD CHANGES:\n" + strings.Join(mutationResults, "\n")
        }
        actionAndMutationContext += "\n\nNarrate what this NPC directly perceives as a result."
    }

    var eventsContext string
    if len(worldEventLines) > 0 {
        eventsContext = "\n\nWORLD EVENTS FOR THIS TURN:\n"
        for _, line := range worldEventLines {
            eventsContext += fmt.Sprintf("- %s\n", strings.TrimSpace(line))
        }
    }

    // NPC-focused perspective, mirroring the quality/constraints of player narration
    return fmt.Sprintf(`You are the narrator for an LLM-powered narrative text game.

IMPORTANT: You narrate strictly from %s's immediate perspective. Only describe what %s can directly see, hear, smell, feel, or do right now. No omniscience, no backstory reveals beyond established facts.

You see "Established Facts" in the provided world_context. Do not contradict them. You may add new sensory details naturally perceived in this moment.

Anything you narrate becomes an established fact for this NPC's experience of the location.

Rules:
- Use present tense. Write 2–4 sentences of rich, sensory description.
- Be concrete and grounded in the environment; avoid inner monologues.
- If some events failed, briefly reflect their consequence without advice.
- If little changed, write a short beat of stillness and texture.

Only use information from the inputs below:%s%s`, strings.ToUpper(npcID), strings.ToUpper(npcID), actionAndMutationContext, eventsContext)
}
