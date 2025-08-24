package perception

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"

    "textadventure/internal/game"
    "textadventure/internal/llm"
)

// GeneratePerceivedEventsForNPC asks the LLM to select which of the given
// world event lines this NPC would reasonably perceive, given the current world state.
// Returns a slice of lines (subset of input), with no inventions.
func GeneratePerceivedEventsForNPC(ctx context.Context, llmService *llm.Service, npcID string, world game.WorldState, worldEventLines []string) ([]string, error) {
    if len(worldEventLines) == 0 {
        return []string{}, nil
    }

    worldCtx := game.BuildWorldContext(world, []string{}, npcID)

    sb := &strings.Builder{}
    fmt.Fprintf(sb, "NPC: %s\n\n", npcID)
    fmt.Fprintf(sb, "WORLD SNAPSHOT (for reasoning):\n%s\n\n", worldCtx)
    fmt.Fprintf(sb, "EVENT LINES:\n%s\n", strings.Join(worldEventLines, "\n"))

    req := llm.JSONCompletionRequest{
        SystemPrompt: `You decide what an NPC perceives in a text adventure.
Given a world snapshot and a list of canonical event lines from this turn, select only the lines the NPC could plausibly perceive.
Rules:
- Only return a JSON array of strings, strictly chosen from the provided event lines.
- Do not invent or paraphrase; copy the exact lines that would be perceived.
- Consider location, proximity, and what could be seen or heard.
- If nothing is perceived, return an empty array []` ,
        UserPrompt:   sb.String(),
        MaxTokens:    150,
    }

    ctx = llm.WithOperationType(ctx, "npc.perceive")
    content, err := llmService.CompleteJSON(ctx, req)
    if err != nil {
        return []string{}, err
    }
    var arr []string
    if jerr := json.Unmarshal([]byte(content), &arr); jerr != nil {
        return []string{}, jerr
    }
    // Ensure we only return exact matches from input (defensive)
    allowed := make(map[string]struct{}, len(worldEventLines))
    for _, l := range worldEventLines {
        allowed[strings.TrimSpace(l)] = struct{}{}
    }
    out := make([]string, 0, len(arr))
    for _, l := range arr {
        if _, ok := allowed[strings.TrimSpace(l)]; ok {
            out = append(out, strings.TrimSpace(l))
        }
    }
    return out, nil
}

