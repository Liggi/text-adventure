package narration

import (
	"fmt"
	"strings"
	
	"textadventure/internal/game/sensory"
)

func buildNarrationPrompt(actionContext string, mutationResults []string, sensoryEvents *sensory.SensoryEventResponse) string {
	var actionAndMutationContext string
	if actionContext != "" {
		actionAndMutationContext = fmt.Sprintf("\n\nACTION THAT JUST OCCURRED:\n%s", actionContext)
		
		if len(mutationResults) > 0 {
			actionAndMutationContext += "\n\nWORLD CHANGES:\n" + strings.Join(mutationResults, "\n")
		}
		
		actionAndMutationContext += "\n\nNarrate the consequences and results of this action."
	}

	var sensoryContext string
	if sensoryEvents != nil && len(sensoryEvents.AuditoryEvents) > 0 {
		sensoryContext = "\n\nSENSORY EVENTS THAT OCCURRED:\n"
		for _, event := range sensoryEvents.AuditoryEvents {
			sensoryContext += fmt.Sprintf("- %s (%s volume) at %s\n", event.Description, event.Volume, event.Location)
		}
		sensoryContext += "\nThese are the ONLY sounds/events that occurred. Do not invent additional sounds or sensory details."
	}

	return fmt.Sprintf(`You are the narrator for a text adventure game. You have complete knowledge of the world state.

Your job: Narrate the consequences and results of player actions with 2-4 sentence vivid narration.

Rules:
- Focus on what happens as a RESULT of the player's action, not the action itself
- The player already knows what they did - tell them what happened because of it
- Describe the world's response: sounds, reactions from NPCs, changes in the environment
- Base narration on mutation results and sensory events that occurred
- If sensory events occurred, incorporate them as the world's response
- When NPCs speak (in sensory events), present their words as dialogue within the narration using quote marks
- DO NOT re-describe what the player just chose to do
- DO NOT invent new sounds, smells, or sensory events beyond what's listed
- If action failed, explain why and suggest alternatives
- Keep responses concise but atmospheric
- ALWAYS use present tense
- Avoid repeating or over-describing things from previous narration%s%s`, actionAndMutationContext, sensoryContext)
}