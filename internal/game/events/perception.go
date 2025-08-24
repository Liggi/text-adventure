package events

import (
    "textadventure/internal/game"
)

// FilterForNPC is deprecated in favor of LLM-driven perception per NPC.
// It is retained temporarily for reference but is no longer used.
func FilterForNPC(npcID string, world game.WorldState, evs []WorldEvent) []WorldEvent {
    npc, ok := world.NPCs[npcID]
    if !ok {
        return []WorldEvent{}
    }
    loc := npc.Location
    out := make([]WorldEvent, 0, len(evs))
    for _, e := range evs {
        // if event location matches npc location (or is empty but actor is npc), include it
        if e.Location == loc || (e.Location == "" && e.Actor == npcID) {
            out = append(out, e)
        }
    }
    return out
}
