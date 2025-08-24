#!/usr/bin/env python3
"""
MCP Server for Text Adventure World State Management

This server provides tools for managing world state mutations in the text adventure game.
It handles player movement, item transfers, inventory management, and world state persistence.
"""

import asyncio
import json
import logging
import sys
from pathlib import Path
from typing import Any, Dict, List, Optional, Union

from mcp.server.fastmcp import FastMCP

# Configure logging to stderr to avoid corrupting JSON-RPC messages
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    stream=sys.stderr
)

logger = logging.getLogger("text-adventure-mcp")

# Initialize FastMCP server
mcp = FastMCP("Text Adventure World State")

# World state file path
WORLD_STATE_FILE = Path(__file__).parent.parent / "world_state.json"

# Default world state
DEFAULT_WORLD_STATE = {
    "player": {
        "location": "foyer",
        "inventory": [],
        "met_npcs": []
    },
    "locations": {
        "foyer": {
            "title": "Old Foyer",
            "description": "Dust hangs in slanted light. A scuffed doormat sits by the threshold and an umbrella stand leans beside a coat rack.",
            "items": ["silver_key", "umbrella_stand", "coat_rack", "side_table", "doormat"],
            "exits": {"north": "study", "east": "library", "west": "kitchen"},
            "door_states": {"north": {"locked": True, "description": "locked oak door"}}
        },
        "study": {
            "title": "Quiet Study", 
            "description": "A heavy oak desk faces a curtained window; papers and a fountain pen sit neatly arranged.",
            "items": ["brass_compass", "fountain_pen", "letter_opener", "sealed_envelope", "blotting_paper"],
            "exits": {"south": "foyer", "up": "attic"},
            "door_states": {}
        },
        "library": {
            "title": "Dusty Library",
            "description": "Narrow aisles between tall shelves. A rolling step ladder rests against the stacks; a small reading table sits under a dim lamp.",
            "items": ["leather_journal", "glass_paperweight", "step_ladder", "reading_glasses", "index_cards", "candle_stub"],
            "exits": {"west": "foyer"},
            "door_states": {}
        },
        "kitchen": {
            "title": "Abandoned Kitchen",
            "description": "Cold stove, hanging utensils, and a faint smell of old spices. A trapdoor’s edges show wear near the center.",
            "items": ["iron_pot", "dented_kettle", "chipped_mug", "hanging_ladle"],
            "exits": {"east": "foyer", "down": "cellar"},
            "door_states": {"down": {"locked": True, "description": "heavy wooden trapdoor"}}
        },
        "attic": {
            "title": "Cramped Attic",
            "description": "Low beams and sloping roof. Dusty crates and a moth-eaten blanket lie near a single round window.",
            "items": ["golden_locket", "moth_eaten_blanket", "wooden_crate", "broken_frame"],
            "exits": {"down": "study"},
            "door_states": {}
        },
        "cellar": {
            "title": "Stone Cellar",
            "description": "Cool stone, low ceiling, and the scent of damp earth. An old wine rack leans against the wall.",
            "items": ["dusty_bottle", "coal_scuttle", "rusted_hook"],
            "exits": {"up": "kitchen"},
            "door_states": {}
        }
    },
    "items": {
        "silver_key": {
            "title": "Silver Key",
            "description": "A tarnished silver key with intricate engravings",
            "can_unlock": ["foyer_north"]
        },
        "brass_compass": {
            "title": "Brass Compass",
            "description": "An antique compass with a needle that spins mysteriously",
            "can_unlock": []
        },
        "leather_journal": {
            "title": "Leather Journal",
            "description": "A weathered journal filled with cryptic notes and sketches",
            "can_unlock": []
        },
        "glass_paperweight": {
            "title": "Glass Paperweight",
            "description": "A smooth, heavy glass paperweight with tiny air bubbles inside",
            "can_unlock": []
        },
        "iron_pot": {
            "title": "Iron Pot",
            "description": "A heavy cast iron pot, blackened with age",
            "can_unlock": []
        },
        "golden_locket": {
            "title": "Golden Locket",
            "description": "An ornate locket that feels warm to the touch",
            "can_unlock": []
        },
        "umbrella_stand": {
            "title": "Umbrella Stand",
            "description": "A wrought-iron stand with a faint ring of rust at its base",
            "can_unlock": []
        },
        "coat_rack": {
            "title": "Coat Rack",
            "description": "A wooden rack with a couple of empty hooks",
            "can_unlock": []
        },
        "side_table": {
            "title": "Side Table",
            "description": "A small table with a scratched surface",
            "can_unlock": []
        },
        "doormat": {
            "title": "Doormat",
            "description": "A coir mat with a faded border",
            "can_unlock": []
        },
        "fountain_pen": {
            "title": "Fountain Pen",
            "description": "A dark lacquer pen with a fine nib",
            "can_unlock": []
        },
        "letter_opener": {
            "title": "Letter Opener",
            "description": "A brass letter opener shaped like a leaf",
            "can_unlock": []
        },
        "sealed_envelope": {
            "title": "Sealed Envelope",
            "description": "A cream envelope sealed with burgundy wax",
            "can_unlock": []
        },
        "blotting_paper": {
            "title": "Blotting Paper",
            "description": "A square of gently stained blotting paper",
            "can_unlock": []
        },
        "step_ladder": {
            "title": "Step Ladder",
            "description": "A small rolling ladder for reaching high shelves",
            "can_unlock": []
        },
        "reading_glasses": {
            "title": "Reading Glasses",
            "description": "Wire-framed glasses in need of a wipe",
            "can_unlock": []
        },
        "index_cards": {
            "title": "Index Cards",
            "description": "A small stack of cards with penciled annotations",
            "can_unlock": []
        },
        "candle_stub": {
            "title": "Candle Stub",
            "description": "A short candle with drips of cooled wax",
            "can_unlock": []
        },
        "dented_kettle": {
            "title": "Dented Kettle",
            "description": "A tin kettle with a dented side",
            "can_unlock": []
        },
        "chipped_mug": {
            "title": "Chipped Mug",
            "description": "A plain white mug with a chip on the rim",
            "can_unlock": []
        },
        "hanging_ladle": {
            "title": "Hanging Ladle",
            "description": "A steel ladle with a looped handle",
            "can_unlock": []
        },
        "moth_eaten_blanket": {
            "title": "Moth-Eaten Blanket",
            "description": "A thin wool blanket with small holes",
            "can_unlock": []
        },
        "wooden_crate": {
            "title": "Wooden Crate",
            "description": "A light crate with a loose slat",
            "can_unlock": []
        },
        "broken_frame": {
            "title": "Broken Frame",
            "description": "A cracked picture frame without its glass",
            "can_unlock": []
        },
        "dusty_bottle": {
            "title": "Dusty Bottle",
            "description": "An unlabeled green bottle coated in dust",
            "can_unlock": []
        },
        "coal_scuttle": {
            "title": "Coal Scuttle",
            "description": "A small metal scuttle with a wooden handle",
            "can_unlock": []
        },
        "rusted_hook": {
            "title": "Rusted Hook",
            "description": "An old iron hook fixed to a beam",
            "can_unlock": []
        }
    },
    "npcs": {
        "elena": {
            "location": "library",
            "debug_color": "35",
            "description": "a woman in her thirties with dark hair loose and slightly disheveled, wearing a simple gray dress",
            "inventory": [],
            "recent_thoughts": [],
            "recent_actions": [],
            "personality": "curious and observant, pragmatic under pressure, empathetic but guarded",
            "backstory": "She has just woken up inside the manor and cannot remember who she is or how she got there.",
            "core_memories": []
        }
    }
}


def load_world_state() -> Dict[str, Any]:
    """Load world state from file, creating default if doesn't exist."""
    try:
        if WORLD_STATE_FILE.exists():
            with open(WORLD_STATE_FILE, 'r') as f:
                return json.load(f)
        else:
            logger.info("Creating default world state file")
            save_world_state(DEFAULT_WORLD_STATE)
            return DEFAULT_WORLD_STATE.copy()
    except Exception as e:
        logger.error(f"Error loading world state: {e}")
        return DEFAULT_WORLD_STATE.copy()


def save_world_state(state: Dict[str, Any]) -> None:
    """Save world state to file."""
    try:
        WORLD_STATE_FILE.parent.mkdir(exist_ok=True)
        with open(WORLD_STATE_FILE, 'w') as f:
            json.dump(state, f, indent=2)
        logger.info("World state saved")
    except Exception as e:
        logger.error(f"Error saving world state: {e}")


@mcp.tool()
async def get_world_state() -> str:
    """Get the current world state for context.
    
    Returns:
        JSON string of the current world state including player location, 
        inventory, room contents, and door states.
    """
    state = load_world_state()
    return json.dumps(state, indent=2)


@mcp.tool()
async def move_player(location: str) -> str:
    """Move the player to a different location.
    
    Args:
        location: The location ID to move the player to (e.g., "study", "foyer")
        
    Returns:
        Success message or error description
    """
    state = load_world_state()
    current_location = state["player"]["location"]
    
    # Validate location exists
    if location not in state["locations"]:
        return f"Error: Location '{location}' does not exist"
    
    # Check if move is allowed (via exits)
    current_exits = state["locations"][current_location].get("exits", {})
    if location not in current_exits.values():
        return f"Error: Cannot move directly from {current_location} to {location}"
    
    # Check if door is locked
    for direction, target in current_exits.items():
        if target == location:
            door_state = state["locations"][current_location].get("door_states", {}).get(direction, {})
            if door_state.get("locked", False):
                return f"Error: The {door_state.get('description', 'door')} is locked"
    
    # Move player
    state["player"]["location"] = location
    save_world_state(state)
    
    return f"Player moved from {current_location} to {location}"


@mcp.tool()
async def move_npc(npc_id: str, location: str) -> str:
    """Move an NPC to a different location.
    
    Args:
        npc_id: The NPC ID to move (e.g., "elena")
        location: The location ID to move the NPC to (e.g., "study", "foyer")
        
    Returns:
        Success message or error description
    """
    state = load_world_state()
    
    if npc_id not in state.get("npcs", {}):
        return f"Error: NPC '{npc_id}' does not exist"
    
    npc = state["npcs"][npc_id]
    current_location = npc["location"]
    
    if location not in state["locations"]:
        return f"Error: Location '{location}' does not exist"
    
    current_exits = state["locations"][current_location].get("exits", {})
    if location not in current_exits.values():
        return f"Error: Cannot move directly from {current_location} to {location}"
    
    for direction, target in current_exits.items():
        if target == location:
            door_state = state["locations"][current_location].get("door_states", {}).get(direction, {})
            if door_state.get("locked", False):
                return f"Error: The {door_state.get('description', 'door')} is locked"
    
    state["npcs"][npc_id]["location"] = location
    save_world_state(state)
    
    return f"NPC {npc_id} moved from {current_location} to {location}"


@mcp.tool()
async def transfer_item(item: str, from_location: str, to_location: str) -> str:
    """Transfer an item from one location to another.
    
    Args:
        item: The item ID to transfer
        from_location: Source location ID (or "player" for inventory)
        to_location: Destination location ID (or "player" for inventory)
        
    Returns:
        Success message or error description
    """
    state = load_world_state()
    
    # Validate item exists
    if item not in state["items"]:
        return f"Error: Item '{item}' does not exist"
    
    # Handle player inventory
    if from_location == "player":
        if item not in state["player"]["inventory"]:
            return f"Error: Item '{item}' not in player inventory"
        state["player"]["inventory"].remove(item)
    # Handle NPC inventory
    elif from_location in state.get("npcs", {}):
        if item not in state["npcs"][from_location].get("inventory", []):
            return f"Error: Item '{item}' not in {from_location}'s inventory"
        state["npcs"][from_location]["inventory"].remove(item)
    else:
        # Validate location exists
        if from_location not in state["locations"]:
            return f"Error: Location '{from_location}' does not exist"
        if item not in state["locations"][from_location]["items"]:
            return f"Error: Item '{item}' not in location '{from_location}'"
        state["locations"][from_location]["items"].remove(item)
    
    # Add to destination
    if to_location == "player":
        state["player"]["inventory"].append(item)
    # Handle NPC inventory
    elif to_location in state.get("npcs", {}):
        if "inventory" not in state["npcs"][to_location]:
            state["npcs"][to_location]["inventory"] = []
        state["npcs"][to_location]["inventory"].append(item)
    else:
        if to_location not in state["locations"]:
            return f"Error: Location '{to_location}' does not exist"
        state["locations"][to_location]["items"].append(item)
    
    save_world_state(state)
    return f"Item '{item}' transferred from {from_location} to {to_location}"


@mcp.tool()
async def add_to_inventory(item: str) -> str:
    """Add an item to the player's inventory from their current location.
    
    Args:
        item: The item ID to pick up
        
    Returns:
        Success message or error description
    """
    state = load_world_state()
    current_location = state["player"]["location"]
    
    # Check if item is in current location
    if item not in state["locations"][current_location]["items"]:
        return f"Error: Item '{item}' is not available in {current_location}"
    
    # Transfer item
    result = await transfer_item(item, current_location, "player")
    
    if result.startswith("Error:"):
        return result
    else:
        return f"Player picked up {item}"


@mcp.tool()
async def remove_from_inventory(item: str) -> str:
    """Remove an item from the player's inventory to their current location.
    
    Args:
        item: The item ID to drop
        
    Returns:
        Success message or error description
    """
    state = load_world_state()
    current_location = state["player"]["location"]
    
    # Check if item is in inventory
    if item not in state["player"]["inventory"]:
        return f"Error: Item '{item}' is not in inventory"
    
    # Transfer item
    result = await transfer_item(item, "player", current_location)
    
    if result.startswith("Error:"):
        return result
    else:
        return f"Player dropped {item} in {current_location}"


@mcp.tool()
async def unlock_door(location: str, direction: str, key_item: str) -> str:
    """Unlock a door using a key from the player's inventory.
    
    Args:
        location: The location where the door is
        direction: The direction of the door (e.g., "north", "south")
        key_item: The key item to use
        
    Returns:
        Success message or error description
    """
    state = load_world_state()
    
    # Validate inputs
    if location not in state["locations"]:
        return f"Error: Location '{location}' does not exist"
    
    if direction not in state["locations"][location].get("door_states", {}):
        return f"Error: No door to the {direction} in {location}"
    
    if key_item not in state["player"]["inventory"]:
        return f"Error: Player does not have {key_item}"
    
    if key_item not in state["items"]:
        return f"Error: Item '{key_item}' does not exist"
    
    # Check if key can unlock this door
    door_id = f"{location}_{direction}"
    key_data = state["items"][key_item]
    if door_id not in key_data.get("can_unlock", []):
        return f"Error: {key_item} cannot unlock this door"
    
    # Unlock the door
    state["locations"][location]["door_states"][direction]["locked"] = False
    save_world_state(state)
    
    return f"Door to the {direction} in {location} has been unlocked with {key_item}"


@mcp.tool()
async def update_npc_memory(npc_id: str, thought: str = "", action: str = "") -> str:
    state = load_world_state()
    
    if npc_id not in state["npcs"]:
        return f"Error: NPC '{npc_id}' does not exist"
    
    npc = state["npcs"][npc_id]
    
    if "recent_thoughts" not in npc:
        npc["recent_thoughts"] = []
    if "recent_actions" not in npc:
        npc["recent_actions"] = []
    
    if thought:
        npc["recent_thoughts"].append(thought)
        if len(npc["recent_thoughts"]) > 4:
            npc["recent_thoughts"] = npc["recent_thoughts"][-4:]
    
    if action:
        npc["recent_actions"].append(action)
        if len(npc["recent_actions"]) > 4:
            npc["recent_actions"] = npc["recent_actions"][-4:]
    
    save_world_state(state)
    
    updates = []
    if thought:
        updates.append(f"thought: '{thought}'")
    if action:
        updates.append(f"action: '{action}'")
    
    if updates:
        return f"Updated {npc_id} memory - {', '.join(updates)}"
    else:
        return f"No updates provided for {npc_id}"


@mcp.tool()
async def configure_npc(npc_id: str, personality: str = "", backstory: str = "", core_memories: str = "") -> str:
    """Configure an NPC's personality, backstory, and core memories.
    
    Args:
        npc_id: The NPC ID to configure (e.g., "elena")
        personality: Brief personality description (e.g., "cautious scholar")
        backstory: Background story explaining who they are
        core_memories: Comma-separated list of important memories
        
    Returns:
        Success message or error description
    """
    state = load_world_state()
    
    if npc_id not in state["npcs"]:
        return f"Error: NPC '{npc_id}' does not exist"
    
    npc = state["npcs"][npc_id]
    updates = []
    
    if personality:
        npc["personality"] = personality
        updates.append("personality")
    
    if backstory:
        npc["backstory"] = backstory
        updates.append("backstory")
    
    if core_memories:
        memory_list = [mem.strip() for mem in core_memories.split(",") if mem.strip()]
        npc["core_memories"] = memory_list
        updates.append("core memories")
    
    if updates:
        save_world_state(state)
        return f"Updated {npc_id}: {', '.join(updates)}"
    else:
        return f"No configuration changes provided for {npc_id}"


@mcp.tool()
async def mark_npc_as_met(npc_id: str) -> str:
    """Mark an NPC as met by the player (for narrative purposes).
    
    Use this when an NPC introduces themselves or the player learns their name,
    so the narrator can refer to them by name instead of description.
    
    Args:
        npc_id: The NPC ID to mark as met (e.g., "elena")
        
    Returns:
        Success message or error description
    """
    state = load_world_state()
    
    if npc_id not in state.get("npcs", {}):
        return f"Error: NPC '{npc_id}' does not exist"
    
    met_npcs = state["player"].get("met_npcs", [])
    if npc_id in met_npcs:
        return f"Player has already met {npc_id}"
    
    met_npcs.append(npc_id)
    state["player"]["met_npcs"] = met_npcs
    save_world_state(state)
    
    return f"Player has now met {npc_id}"


if __name__ == "__main__":
    # Run the server (FastMCP manages its own event loop)
    mcp.run()
