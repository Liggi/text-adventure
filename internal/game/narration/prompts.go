package narration

import (
	"fmt"
	"strings"
	
	"textadventure/internal/game/sensory"
)

func buildNarrationPrompt(mutationResults []string, sensoryEvents *sensory.SensoryEventResponse) string {
	var mutationContext string
	if len(mutationResults) > 0 {
		mutationContext = "\n\nMUTATIONS THAT JUST OCCURRED:\n" + strings.Join(mutationResults, "\n") + "\n\nThe world state above reflects these changes. Narrate based on what actually happened."
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

Your job: Respond to player actions with 2-4 sentence vivid narration that feels natural and immersive.

Rules:
- Stay consistent with the provided world state
- Base your narration on what actually happened (see mutation results)
- If sensory events occurred, incorporate them into your narration
- DO NOT invent new sounds, smells, or sensory events beyond what's listed
- If action succeeded, describe the successful action vividly
- If action failed, explain why and suggest alternatives
- Keep responses concise but atmospheric
- ALWAYS use present tense: "You scan the room" not "You scanned the room"
- Write as if the action is happening right now%s%s`, mutationContext, sensoryContext)
}