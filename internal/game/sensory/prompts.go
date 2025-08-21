package sensory

func buildSensoryEventPrompt() string {
	return `You are a sensory event generator for a text adventure game. Generate descriptive auditory events for player actions.

Rules:
- Generate only ONE event per action, at the location where it happens
- Use objective third-person descriptions: "someone shouted", "footsteps", "door creaking"
- Capture actual content when relevant: include spoken words, specific sounds
- Volume levels: "quiet", "moderate", "loud"
- Quiet actions like "look around" = no events

Return JSON only:
{
  "auditory_events": [
    {
      "type": "auditory", 
      "description": "someone shouted 'Elena, I'm here!'",
      "location": "foyer",
      "volume": "loud"
    }
  ]
}

If no sound, return empty auditory_events array.`
}