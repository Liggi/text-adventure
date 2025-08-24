package facts

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"textadventure/internal/game"
	"textadventure/internal/llm"

	"go.opentelemetry.io/otel"
)

type FactAttribution struct {
	LocationFacts map[string][]string `json:"location_facts"`
	ItemFacts     map[string][]string `json:"item_facts"`
	NPCFacts      map[string][]string `json:"npc_facts"`
	Skipped       []string            `json:"skipped"`
}

func AttributeFacts(ctx context.Context, llmService *llm.Service, extractedFacts []string, worldState *game.WorldState) (*FactAttribution, error) {
	tracer := otel.Tracer("facts")
	ctx, span := tracer.Start(ctx, "facts.attribute")
	defer span.End()

	if len(extractedFacts) == 0 {
		return &FactAttribution{
			LocationFacts: make(map[string][]string),
			ItemFacts:     make(map[string][]string),
			NPCFacts:      make(map[string][]string),
			Skipped:       []string{},
		}, nil
	}

	systemPrompt := buildAttributionPrompt(worldState, extractedFacts)

	userPrompt := fmt.Sprintf("Attribute these extracted facts: %s", strings.Join(extractedFacts, ", "))

	response, err := llmService.CompleteJSON(ctx, llm.JSONCompletionRequest{
		SystemPrompt:    systemPrompt,
		UserPrompt:      userPrompt,
		MaxTokens:       2000,
		Model:           "gpt-5-mini",
		ReasoningEffort: "minimal",
	})
	if err != nil {
		return nil, fmt.Errorf("LLM attribution failed: %w", err)
	}

	// Parse the JSON response
	var attribution FactAttribution
	if err := json.Unmarshal([]byte(response), &attribution); err != nil {
		return nil, fmt.Errorf("failed to parse attribution response: %w", err)
	}

	// Initialize maps if they're nil
	if attribution.LocationFacts == nil {
		attribution.LocationFacts = make(map[string][]string)
	}
	if attribution.ItemFacts == nil {
		attribution.ItemFacts = make(map[string][]string)
	}
	if attribution.NPCFacts == nil {
		attribution.NPCFacts = make(map[string][]string)
	}
	if attribution.Skipped == nil {
		attribution.Skipped = []string{}
	}

	return &attribution, nil
}

func buildAttributionPrompt(worldState *game.WorldState, extractedFacts []string) string {
	var contextBuilder strings.Builder
	
	contextBuilder.WriteString("You are attributing facts extracted from player narration to the correct entities in a text adventure game.\n\n")

	contextBuilder.WriteString("CURRENT WORLD CONTEXT:\n")
	
	currentLocation := worldState.Locations[worldState.Location]
	contextBuilder.WriteString(fmt.Sprintf("Player is currently in: %s\n", currentLocation.Name))
	if len(currentLocation.Facts) > 0 {
		contextBuilder.WriteString(fmt.Sprintf("Existing location facts: %v\n", currentLocation.Facts))
	}

	contextBuilder.WriteString("\nAVAILABLE ENTITIES:\n")
	contextBuilder.WriteString("Locations:\n")
	for locID, loc := range worldState.Locations {
		contextBuilder.WriteString(fmt.Sprintf("- %s (%s): existing facts %v\n", locID, loc.Name, loc.Facts))
	}

	contextBuilder.WriteString("\nNPCs:\n")
	for npcID, npc := range worldState.NPCs {
		contextBuilder.WriteString(fmt.Sprintf("- %s: location=%s, existing facts %v\n", npcID, npc.Location, npc.Facts))
	}

	contextBuilder.WriteString("\nItems: (items are created dynamically - you can reference any item mentioned in the facts)\n")

	contextBuilder.WriteString(`
ATTRIBUTION RULES:
1. **Physical/architectural details** about the space → location_facts
2. **Object-specific details** (appearance, properties) → item_facts (create item if needed)
3. **Character details** (appearance, behavior, traits) → npc_facts
4. **Skip facts** that are semantically similar to existing facts
5. **Permanent facts only** - skip temporary states, emotions, positions

SEMANTIC DEDUPLICATION:
- "dusty atmosphere" vs "dust particles in air" → SKIP the second
- "slanted light" vs "light enters at an angle" → SKIP the second  
- "umbrella stand has dull ring" vs "umbrella stand rim is worn" → SKIP the second

OUTPUT FORMAT:
Return JSON with this exact structure:
{
  "location_facts": {"location_id": ["fact1", "fact2"]},
  "item_facts": {"item_id": ["fact1", "fact2"]},  
  "npc_facts": {"npc_id": ["fact1", "fact2"]},
  "skipped": ["fact (reason: similar to existing 'other fact')"]
}

Only include entities that have facts to add. Use empty objects {} for sections with no facts.`)

	return contextBuilder.String()
}