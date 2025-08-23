package sensory

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"

    "textadventure/internal/debug"
    "textadventure/internal/game"
    "textadventure/internal/llm"
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
func GenerateSensoryEvents(ctx context.Context, llmService *llm.Service, userInput string, successfulMutations []string, world game.WorldState, debugLogger *debug.Logger, actingNPCID ...string) (*SensoryEventResponse, error) {
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
	
	req := llm.JSONCompletionRequest{
		SystemPrompt: buildSensoryEventPrompt(),
		UserPrompt:   contextMsg,
		MaxTokens:    400,
	}

    ctx = llm.WithOperationType(ctx, "sensory.generate")
    ctx = llm.WithGameContext(ctx, map[string]interface{}{
        "player_location": world.Location,
        "mutation_count":  len(successfulMutations),
    })
    content, err := llmService.CompleteJSON(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("sensory event generation failed: %w", err)
	}
	
	var eventResp SensoryEventResponse
	if err := json.Unmarshal([]byte(content), &eventResp); err != nil {
		debugLogger.Printf("JSON unmarshal failed: %v", err)
		return &SensoryEventResponse{AuditoryEvents: []SensoryEvent{}}, nil
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
