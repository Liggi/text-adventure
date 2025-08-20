package sensory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/sashabaranov/go-openai"

	"textadventure/internal/game"
)

// SensoryEvent represents a sensory event that occurs in the game world
type SensoryEvent struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Location    string `json:"location"`
	Volume      string `json:"volume,omitempty"`
}

// SensoryEventResponse contains all sensory events generated for an action
type SensoryEventResponse struct {
	AuditoryEvents []SensoryEvent `json:"auditory_events"`
}

// GenerateSensoryEvents generates sensory events (sounds, etc.) for player or NPC actions
func GenerateSensoryEvents(client *openai.Client, userInput string, successfulMutations []string, world game.WorldState, debug bool, actingNPCID ...string) (*SensoryEventResponse, error) {
	systemPrompt := `You are a sensory event generator for a text adventure game. Generate descriptive auditory events for player actions.

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

	var actionLabel string
	var currentLocation string
	
	if len(actingNPCID) > 0 && actingNPCID[0] != "" {
		actionLabel = fmt.Sprintf("NPC %s ACTION", strings.ToUpper(actingNPCID[0]))
		if npc, exists := world.NPCs[actingNPCID[0]]; exists {
			currentLocation = npc.Location
		} else {
			currentLocation = world.Location
		}
	} else {
		actionLabel = "Player action"
		currentLocation = world.Location
	}
	
	var contextMsg string
	if len(successfulMutations) > 0 {
		mutationContext := "Successful mutations:\n" + strings.Join(successfulMutations, "\n")
		contextMsg = fmt.Sprintf("%s: %s\nCurrent location: %s\n\n%s", actionLabel, userInput, currentLocation, mutationContext)
	} else {
		contextMsg = fmt.Sprintf("%s: %s\nCurrent location: %s", actionLabel, userInput, currentLocation)
	}
	
	req := openai.ChatCompletionRequest{
		Model: "gpt-5-2025-08-07",
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: contextMsg,
			},
		},
		MaxCompletionTokens: 400,
		ReasoningEffort:     "minimal",
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	}

	if debug {
		log.Printf("=== SENSORY EVENT GENERATION START ===")
		log.Printf("Action: %q", userInput)
		log.Printf("Context message: %s", contextMsg)
		log.Printf("System prompt length: %d chars", len(systemPrompt))
	}

	resp, err := client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		if debug {
			log.Printf("SENSORY EVENT API ERROR: %v", err)
		}
		return nil, fmt.Errorf("sensory event generation failed: %w", err)
	}

	if debug {
		log.Printf("API Response - Choices length: %d", len(resp.Choices))
		if len(resp.Choices) > 0 {
			log.Printf("Response choice 0 - Finish reason: %s", resp.Choices[0].FinishReason)
		}
	}

	var eventResp SensoryEventResponse
	content := resp.Choices[0].Message.Content
	
	if debug {
		log.Printf("Raw sensory event response length: %d", len(content))
		log.Printf("Raw sensory event response: %q", content)
	}
	
	if err := json.Unmarshal([]byte(content), &eventResp); err != nil {
		if debug {
			log.Printf("JSON unmarshal failed: %v", err)
			log.Printf("Returning empty sensory events response")
		}
		return &SensoryEventResponse{AuditoryEvents: []SensoryEvent{}}, nil
	}

	if debug {
		log.Printf("Generated %d sensory events", len(eventResp.AuditoryEvents))
		for i, event := range eventResp.AuditoryEvents {
			log.Printf("  Event %d: %s (volume: %s, location: %s)", i, event.Description, event.Volume, event.Location)
		}
		log.Printf("=== SENSORY EVENT GENERATION END ===")
	}

	return &eventResp, nil
}

// CalculateRoomDistance calculates the shortest path distance between two locations
func CalculateRoomDistance(fromLocation, toLocation string, locations map[string]game.LocationInfo) int {
	if fromLocation == toLocation {
		return 0
	}
	
	// BFS to find shortest path
	visited := make(map[string]bool)
	queue := []struct {
		location string
		distance int
	}{{fromLocation, 0}}
	
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		
		if visited[current.location] {
			continue
		}
		visited[current.location] = true
		
		if current.location == toLocation {
			return current.distance
		}
		
		// Add all connected rooms to queue
		if loc, exists := locations[current.location]; exists {
			for _, destination := range loc.Exits {
				if !visited[destination] {
					queue = append(queue, struct {
						location string
						distance int
					}{destination, current.distance + 1})
				}
			}
		}
	}
	
	return -1 // No path found
}

// ApplyVolumeDecay applies volume decay based on distance for sound propagation
func ApplyVolumeDecay(originalVolume string, distance int) string {
	if distance < 0 {
		return "" // No path, can't hear
	}
	
	switch originalVolume {
	case "loud":
		switch distance {
		case 0: return "loudly"
		case 1: return "moderately"  
		case 2: return "faintly"
		default: return "" // Too far
		}
	case "moderate":
		switch distance {
		case 0: return "moderately"
		case 1: return "faintly"
		default: return "" // Too far
		}
	case "quiet":
		switch distance {
		case 0: return "quietly"
		default: return "" // Too far
		}
	default:
		return ""
	}
}