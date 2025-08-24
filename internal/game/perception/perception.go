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
- Event lines may include tags of the form "Actor@location: ...". Prefer selecting lines where the location matches the NPC's current room.
- Consider location, proximity, and what could be seen or heard (e.g., speech may carry to nearby rooms; be conservative).
- If nothing is perceived, return an empty array []` ,
        UserPrompt:   sb.String(),
        MaxTokens:    150,
    }

    ctx = llm.WithOperationType(ctx, "npc.perceive")
    content, err := llmService.CompleteJSON(ctx, req)
    if err != nil {
        return []string{}, err
    }

    // Robust parsing: handle direct arrays, object-wrapped arrays, and empty content
    var arr []string
    if strings.TrimSpace(content) == "" {
        arr = []string{}
    } else if jerr := json.Unmarshal([]byte(content), &arr); jerr != nil {
        // Try common object-wrapped formats
        var obj map[string]interface{}
        if oerr := json.Unmarshal([]byte(content), &obj); oerr == nil {
            for _, key := range []string{"perceived", "events", "lines", "results", "items"} {
                if v, ok := obj[key]; ok {
                    if a, ok := v.([]interface{}); ok {
                        tmp := make([]string, 0, len(a))
                        for _, it := range a {
                            if s, ok := it.(string); ok && strings.TrimSpace(s) != "" {
                                tmp = append(tmp, strings.TrimSpace(s))
                            }
                        }
                        arr = tmp
                        break
                    }
                }
            }
            // If LLM returned an error object, treat as empty selection
            if len(arr) == 0 {
                if _, hasErr := obj["error"]; hasErr {
                    arr = []string{}
                }
            }
        }
        // If still not parsed, fall back to empty set (do not hard fail; deterministic additions later apply)
        if arr == nil {
            arr = []string{}
        }
    }
    // Ensure we only return exact matches from input (defensive)
    allowed := make(map[string]struct{}, len(worldEventLines))
    for _, l := range worldEventLines {
        allowed[strings.TrimSpace(l)] = struct{}{}
    }
    selected := make(map[string]struct{})
    out := make([]string, 0, len(arr))
    for _, l := range arr {
        s := strings.TrimSpace(l)
        if _, ok := allowed[s]; ok {
            if _, seen := selected[s]; !seen {
                selected[s] = struct{}{}
                out = append(out, s)
            }
        }
    }

    // Deterministic addition: include speech-like attempts from adjacent rooms
    npcLoc := world.NPCs[npcID].Location
    adj := make(map[string]struct{})
    if loc, ok := world.Locations[npcLoc]; ok {
        for _, v := range loc.Exits { adj[v] = struct{}{} }
    }
    for _, l := range worldEventLines {
        s := strings.TrimSpace(l)
        at := strings.Index(s, "@")
        colon := strings.Index(s, ":")
        if at > 0 && colon > at {
            locTag := strings.TrimSpace(s[at+1 : colon])
            content := strings.TrimSpace(s[colon+1:])
            lc := strings.ToLower(content)
            if _, ok := allowed[s]; ok {
                if locTag == npcLoc {
                    // already same room, it should have been selected by LLM if relevant; keep union semantics
                    if _, seen := selected[s]; !seen && isSpeechLike(lc) {
                        selected[s] = struct{}{}
                        out = append(out, s)
                    }
                    continue
                }
                if _, isAdj := adj[locTag]; isAdj && isSpeechLike(lc) {
                    if _, seen := selected[s]; !seen {
                        selected[s] = struct{}{}
                        out = append(out, s)
                    }
                }
            }
        }
    }

    return out, nil
}

// isSpeechLike determines if an event content likely represents audible speech/shouting.
func isSpeechLike(lc string) bool {
    if strings.Contains(lc, "shout") || strings.Contains(lc, "yell") || strings.Contains(lc, "call out") || strings.Contains(lc, "say ") || strings.Contains(lc, "say:") || strings.Contains(lc, "\"") {
        return true
    }
    return false
}
