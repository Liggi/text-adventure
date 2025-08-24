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
            "name": "Old Foyer",
            "facts": [],
            "exits": {"north": "study", "east": "library", "west": "kitchen"},
            "door_states": {"north": {"locked": True, "description": "locked oak door"}}
        },
        "study": {
            "name": "Quiet Study", 
            "facts": [],
            "exits": {"south": "foyer", "up": "attic"},
            "door_states": {}
        },
        "library": {
            "name": "Dusty Library",
            "facts": [],
            "exits": {"west": "foyer"},
            "door_states": {}
        },
        "kitchen": {
            "name": "Abandoned Kitchen",
            "facts": [],
            "exits": {"east": "foyer", "down": "cellar"},
            "door_states": {"down": {"locked": True, "description": "heavy wooden trapdoor"}}
        },
        "attic": {
            "name": "Cramped Attic",
            "facts": [],
            "exits": {"down": "study"},
            "door_states": {}
        },
        "cellar": {
            "name": "Stone Cellar",
            "facts": [],
            "exits": {"up": "kitchen"},
            "door_states": {}
        }
    },
    "items": {},
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


@mcp.tool()
async def create_item(item_id: str, name: str, location: str, initial_facts: Optional[List[str]] = None) -> str:
    """Create a new item in the world.
    
    Args:
        item_id: Unique identifier for the item (e.g., "silver_key")
        name: Human-readable name (e.g., "Silver Key")  
        location: Where the item is located (location_id, "player", or npc_id)
        initial_facts: Optional list of initial facts about the item
        
    Returns:
        Success message or error description
    """
    state = load_world_state()
    
    if item_id in state.get("items", {}):
        return f"Error: Item '{item_id}' already exists"
    
    # Validate location exists
    if location != "player" and location not in state.get("locations", {}) and location not in state.get("npcs", {}):
        return f"Error: Location '{location}' does not exist"
    
    state["items"][item_id] = {
        "name": name,
        "facts": initial_facts or [],
        "location": location,
        "can_unlock": []
    }
    
    save_world_state(state)
    return f"Created item '{name}' ({item_id}) at {location}"


@mcp.tool()
async def create_npc(npc_id: str, name: str, location: str, initial_facts: Optional[List[str]] = None) -> str:
    """Create a new NPC in the world.
    
    Args:
        npc_id: Unique identifier for the NPC (e.g., "elena")
        name: Human-readable name (e.g., "Elena")
        location: Location where NPC starts
        initial_facts: Optional list of initial facts about the NPC
        
    Returns:
        Success message or error description
    """
    state = load_world_state()
    
    if npc_id in state.get("npcs", {}):
        return f"Error: NPC '{npc_id}' already exists"
    
    if location not in state.get("locations", {}):
        return f"Error: Location '{location}' does not exist"
    
    state["npcs"][npc_id] = {
        "name": name,
        "location": location,
        "debug_color": "37",
        "facts": initial_facts or [],
        "inventory": [],
        "recent_thoughts": [],
        "recent_actions": [],
        "personality": "",
        "backstory": "",
        "memories": []
    }
    
    save_world_state(state)
    return f"Created NPC '{name}' ({npc_id}) at {location}"


@mcp.tool() 
async def create_location(location_id: str, name: str, exits: Optional[Dict[str, str]] = None) -> str:
    """Create a new location in the world.
    
    Args:
        location_id: Unique identifier for the location (e.g., "secret_room")
        name: Human-readable name (e.g., "Secret Room")
        exits: Optional dictionary of exits {"direction": "location_id"}
        
    Returns:
        Success message or error description
    """
    state = load_world_state()
    
    if location_id in state.get("locations", {}):
        return f"Error: Location '{location_id}' already exists"
    
    state["locations"][location_id] = {
        "name": name,
        "facts": [],
        "exits": exits or {},
        "door_states": {}
    }
    
    save_world_state(state)
    return f"Created location '{name}' ({location_id})"


@mcp.tool()
async def add_location_facts(location_id: str, new_facts: List[str]) -> str:
    """Add facts to a location.
    
    Args:
        location_id: The location to add facts to
        new_facts: List of facts to add
        
    Returns:
        Success message or error description
    """
    state = load_world_state()
    
    if location_id not in state.get("locations", {}):
        return f"Error: Location '{location_id}' does not exist"
    
    location = state["locations"][location_id]
    existing_facts = location.get("facts", [])
    
    # Add all facts - deduplication handled by LLM at attribution level
    existing_facts.extend(new_facts)
    location["facts"] = existing_facts
    save_world_state(state)
    
    return f"Added {len(new_facts)} facts to {location_id}: {new_facts}"


@mcp.tool()
async def add_item_facts(item_id: str, new_facts: List[str]) -> str:
    """Add facts to an item.
    
    Args:
        item_id: The item to add facts to
        new_facts: List of facts to add
        
    Returns:
        Success message or error description
    """
    state = load_world_state()
    
    if item_id not in state.get("items", {}):
        return f"Error: Item '{item_id}' does not exist"
    
    item = state["items"][item_id]
    existing_facts = item.get("facts", [])
    
    # Add all facts - deduplication handled by LLM at attribution level
    existing_facts.extend(new_facts)
    item["facts"] = existing_facts
    save_world_state(state)
    
    return f"Added {len(new_facts)} facts to {item_id}: {new_facts}"


@mcp.tool()
async def add_npc_facts(npc_id: str, new_facts: List[str]) -> str:
    """Add facts to an NPC.
    
    Args:
        npc_id: The NPC to add facts to
        new_facts: List of facts to add
        
    Returns:
        Success message or error description
    """
    state = load_world_state()
    
    if npc_id not in state.get("npcs", {}):
        return f"Error: NPC '{npc_id}' does not exist"
    
    npc = state["npcs"][npc_id]
    existing_facts = npc.get("facts", [])
    
    # Simple deduplication - exact string matches only
    added_facts = []
    for fact in new_facts:
        if fact not in existing_facts:
            existing_facts.append(fact)
            added_facts.append(fact)
    
    npc["facts"] = existing_facts
    save_world_state(state)
    
    if added_facts:
        return f"Added {len(added_facts)} facts to {npc_id}: {added_facts}"
    else:
        return f"No new facts added to {npc_id} (all were duplicates)"


if __name__ == "__main__":
    # Run the server (FastMCP manages its own event loop)
    mcp.run()
