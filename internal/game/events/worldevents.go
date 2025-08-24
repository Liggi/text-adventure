package events

import (
    "fmt"
    "time"
)

// WorldEventType represents the canonical type of an in-game event.
type WorldEventType string

const (
    EventMovement      WorldEventType = "movement"
    EventItemTransfer  WorldEventType = "item_transfer"
    EventInventory     WorldEventType = "inventory"
    EventSpeech        WorldEventType = "speak"
    EventSound         WorldEventType = "sound"
    EventStateChange   WorldEventType = "state_change"
    EventMutation      WorldEventType = "mutation"
)

// WorldEvent is the canonical record of something that happened in the world.
type WorldEvent struct {
    ID        string                 `json:"id"`
    Type      WorldEventType         `json:"type"`
    Actor     string                 `json:"actor,omitempty"`
    Target    string                 `json:"target,omitempty"`
    Location  string                 `json:"location,omitempty"`
    Content   string                 `json:"content,omitempty"`
    Meta      map[string]interface{} `json:"meta,omitempty"`
    Timestamp time.Time              `json:"timestamp"`
}

// Mutation is a lightweight representation of a planned mutation.
type Mutation struct {
    Tool string
    Args map[string]interface{}
}

// FromMutations creates a best-effort set of world events from a list of mutations.
// This is intentionally conservative and schema-stable; specific tools are mapped
// to canonical event types, otherwise a generic mutation event is emitted.
func FromMutations(actor string, location string, muts []Mutation) []WorldEvent {
    ts := time.Now()
    var out []WorldEvent
    for i, m := range muts {
        ev := WorldEvent{
            ID:        fmt.Sprintf("ev_%d_%d", ts.UnixNano(), i),
            Type:      EventMutation,
            Actor:     actor,
            Location:  location,
            Meta:      map[string]interface{}{"tool": m.Tool, "args": m.Args},
            Timestamp: ts,
        }
        switch m.Tool {
        case "move_player":
            ev.Type = EventMovement
            if loc, ok := m.Args["location"].(string); ok {
                ev.Target = loc
                ev.Content = fmt.Sprintf("%s moved to %s", actor, loc)
            }
        case "move_npc":
            ev.Type = EventMovement
            if to, ok := m.Args["to"].(string); ok {
                ev.Target = to
                ev.Content = fmt.Sprintf("%s moved to %s", actor, to)
            }
        case "transfer_item":
            ev.Type = EventItemTransfer
            item, _ := m.Args["item"].(string)
            to, _ := m.Args["to_location"].(string)
            from, _ := m.Args["from_location"].(string)
            ev.Content = fmt.Sprintf("%s transferred %s from %s to %s", actor, item, from, to)
            ev.Target = item
        case "add_to_inventory", "remove_from_inventory":
            ev.Type = EventInventory
            item, _ := m.Args["item"].(string)
            ev.Content = fmt.Sprintf("%s %s %s", actor, m.Tool, item)
            ev.Target = item
        }
        out = append(out, ev)
    }
    return out
}

