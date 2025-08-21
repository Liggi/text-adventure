package director

import (
	"fmt"
	
	"textadventure/internal/game"
)

func buildDirectorPrompt(toolDescriptions string, world game.WorldState, gameHistory []string, actionLabel string, actingNPCID string) string {
	var actorInstructions string
	var movementInstructions string
	var exampleDestination string
	
	if actingNPCID != "" {
		actorInstructions = fmt.Sprintf("- For NPC picking up items: use transfer_item to move from location to %s", actingNPCID)
		movementInstructions = fmt.Sprintf("- For NPC movement: use move_npc tool with npc_id \"%s\"", actingNPCID)
		exampleDestination = actingNPCID
	} else {
		actorInstructions = "- For picking up items: use transfer_item to move from location to player, then add_to_inventory"
		movementInstructions = "- For player movement: use move_player tool"
		exampleDestination = "player"
	}
	
	return fmt.Sprintf(`You are the Director of a text adventure game. Your role is to understand player intent and generate the specific world mutations needed to make it happen.

%s

WORLD STATE CONTEXT:
%s

RULES:
- Parse the %s and decide what world mutations are needed
- Generate JSON array of mutations using the available tools
- Be conservative - only generate mutations that directly relate to the stated action
%s
%s
- For dropping items: use remove_from_inventory, then transfer_item to move to current location
- For examining/looking: usually no mutations needed
- NPCs can only affect items at their current location or their own movement

Return JSON format:
{
  "mutations": [
    {"tool": "move_player", "args": {"location": "kitchen"}},
    {"tool": "transfer_item", "args": {"item": "key", "from_location": "foyer", "to_location": "%s"}}
  ]
}

If no mutations needed, return empty mutations array.`, toolDescriptions, game.BuildWorldContext(world, gameHistory, actingNPCID), actionLabel, movementInstructions, actorInstructions, exampleDestination)
}