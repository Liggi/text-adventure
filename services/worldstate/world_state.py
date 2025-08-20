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
        "inventory": []
    },
    "locations": {
        "foyer": {
            "title": "Old Foyer",
            "description": "A dusty foyer with motes drifting in shafts of light",
            "items": ["silver_key"],
            "exits": {"north": "study"},
            "door_states": {"north": {"locked": True, "description": "locked oak door"}}
        },
        "study": {
            "title": "Quiet Study", 
            "description": "A quiet study with a heavy oak desk",
            "items": [],
            "exits": {"south": "foyer"},
            "door_states": {}
        }
    },
    "items": {
        "silver_key": {
            "title": "Silver Key",
            "description": "A tarnished silver key",
            "can_unlock": ["foyer_north"]
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


if __name__ == "__main__":
    # Run the server (FastMCP manages its own event loop)
    mcp.run()
