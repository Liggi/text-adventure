package facts

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"textadventure/internal/llm"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

func ExtractLocationFacts(ctx context.Context, llmService *llm.Service, narrationText, locationID string, existingFacts []string) ([]string, error) {
	if strings.TrimSpace(narrationText) == "" {
		return []string{}, nil
	}

	tracer := otel.Tracer("facts")
	ctx, span := tracer.Start(ctx, "facts.extract")
	defer span.End()

	systemPrompt := `Extract permanent, canonical facts about the location from this narration as directly experienced by the observer.

IMPORTANT: Only extract facts that represent what the observer directly observed or experienced. This narration describes things from a first-person present perspective - the facts you extract become the observer's established knowledge about this space.

Write facts as short, precise descriptions WITHOUT repeating the location name:
- GOOD: "has slanted light", "smells of old paper", "doormat is scuffed"
- BAD: "Old Foyer has slanted light", "The Old Foyer smells of old paper"

INCLUDE physical/architectural details that the player directly perceived:
- Physical features: "has tall windows", "made of oak", "dusty atmosphere"  
- Architectural elements: "stone floors", "vaulted ceiling", "narrow doorway"
- Inherent properties: "door creaks", "rusty hinges", "faded wallpaper"
- Atmospheric qualities: "dim lighting", "musty smell", "echoes sounds"

EXCLUDE temporary states and actions:
- Current actions: "Elena walks carefully" → NO
- Positional info: "Elena is near window" → NO  
- Temporary conditions: "door is open" → NO
- Time-specific states: "morning light streams" → NO

AVOID semantic duplicates of existing facts:
- If existing facts include "dusty atmosphere", don't extract "dust particles in air" or "covered in dust"
- If existing facts include "wooden door", don't extract "made of wood" for the same door
- Look for semantic similarity, not just exact text matches

Return a JSON array of strings. Each fact should be maximally granular and concise.
Extract each detail as a separate fact. Only extract what the observer has genuinely perceived.`

	existingFactsSection := ""
	if len(existingFacts) > 0 {
		existingFactsSection = fmt.Sprintf(`

Existing Facts (DO NOT duplicate):
%s`, strings.Join(existingFacts, "\n"))
	}

	userPrompt := fmt.Sprintf(`Location: %s

Narration: %s%s

Extract permanent canonical facts about this location:`, locationID, narrationText, existingFactsSection)

	req := llm.JSONCompletionRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    150,
		Model:        "gpt-5-mini",
	}

	ctx = llm.WithOperationType(ctx, "facts.extract")
	span.SetAttributes(
		attribute.String("langfuse.observation.type", "generation"),
		attribute.String("facts.narration_input", narrationText),
		attribute.String("facts.location_id", locationID),
	)

	content, err := llmService.CompleteJSON(ctx, req)
	if err != nil {
		span.RecordError(err)
		return []string{}, fmt.Errorf("fact extraction failed: %w", err)
	}

	var facts []string
	
	// Try to parse as array first
	if jerr := json.Unmarshal([]byte(content), &facts); jerr != nil {
		// If array parsing fails, try parsing as object with common keys
		var objResponse map[string]interface{}
		if objErr := json.Unmarshal([]byte(content), &objResponse); objErr != nil {
			span.RecordError(jerr)
			return []string{}, fmt.Errorf("fact extraction JSON parse failed: %w", jerr)
		}
		
		// Look for array in common object keys
		for _, key := range []string{"facts", "extracted_facts", "results", "items"} {
			if val, exists := objResponse[key]; exists {
				if arr, ok := val.([]interface{}); ok {
					facts = make([]string, 0, len(arr))
					for _, item := range arr {
						if str, ok := item.(string); ok {
							facts = append(facts, str)
						}
					}
					break
				}
			}
		}
	}

	cleanFacts := make([]string, 0, len(facts))
	for _, fact := range facts {
		fact = strings.TrimSpace(fact)
		if fact != "" {
			cleanFacts = append(cleanFacts, fact)
		}
	}

	span.SetAttributes(
		attribute.Int("facts.extracted_count", len(cleanFacts)),
	)

	return cleanFacts, nil
}
