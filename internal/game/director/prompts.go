package director

import (
	"fmt"
	"strings"
	
	"textadventure/internal/game"
)

func buildDirectorPrompt(toolDescriptions string, world game.WorldState, gameHistory []string, actionLabel string, actingNPCID string) string {
    var movementGuideline string
    var pickupGuidelines string
    var exampleDestination string

    if actingNPCID != "" {
        movementGuideline = fmt.Sprintf("- Movement: use move_npc with npc_id=\"%s\".", actingNPCID)
        pickupGuidelines = fmt.Sprintf("- Pick up item: use transfer_item from location → %s.\n- If NPC introduces themselves: use mark_npc_as_met with npc_id=\"%s\".", actingNPCID, actingNPCID)
        exampleDestination = actingNPCID
    } else {
        movementGuideline = "- Movement: use move_player."
        pickupGuidelines = "- Pick up item: use transfer_item from location → player, then add_to_inventory.\n- If meeting someone who gives their name: use mark_npc_as_met with their npc_id."
        exampleDestination = "player"
    }

    return fmt.Sprintf(`You are the Director of a text adventure game. Generate only the world mutations required to fulfill the user's intent.

<available_tools>
%s
</available_tools>

<context>
%s
</context>

<guidelines>
- Interpret the %s and produce only necessary mutations using the available tools.
- Output strictly as a JSON object: {"mutations": [ ... ]} — no extra text.
- Be conservative; avoid speculative or unrelated changes.
%s
%s
- Drop item: remove_from_inventory, then transfer_item to current location.
- Examine/look at environment: usually no mutations needed.
- Examine/look at NPCs or specific items: may need mutations to trigger detailed descriptions or NPC reactions.
- NPCs may only affect items at their location or move themselves.
</guidelines>

<example_output>
{"mutations": [
  {"tool": "move_player", "args": {"location": "kitchen"}},
  {"tool": "transfer_item", "args": {"item": "key", "from_location": "foyer", "to_location": "%s"}}
]}
</example_output>
`, toolDescriptions, game.BuildWorldContext(world, gameHistory, actingNPCID), actionLabel, movementGuideline, pickupGuidelines, exampleDestination)
}

func getCoreDirectorTools() string {
	coreTools := []string{
		"move_player(location: string) - Move the player to a specific location",
		"move_npc(npc_id: string, location: string) - Move an NPC to a specific location", 
		"transfer_item(item: string, from_location: string, to_location: string) - Move an item between locations or entities",
		"add_to_inventory(item: string) - Add an item from current location to player's inventory",
		"remove_from_inventory(item: string) - Remove an item from player's inventory to current location",
		"mark_npc_as_met(npc_id: string) - Mark that the player has met and learned an NPC's name",
	}
	
	return strings.Join(coreTools, "\n")
}
