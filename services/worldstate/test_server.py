#!/usr/bin/env python3
"""
Test script for the MCP server to verify tools work correctly.
"""

import asyncio
import json
import sys
from pathlib import Path

# Add current directory to path to import our server
sys.path.append(str(Path(__file__).parent))

from world_state import (
    get_world_state, move_player, transfer_item, 
    add_to_inventory, remove_from_inventory, unlock_door
)


async def test_basic_flow():
    """Test the basic game flow: look, take key, unlock door, move."""
    
    print("=== Testing MCP Server Tools ===\n")
    
    print("Getting initial world state:")
    state = await get_world_state()
    print(state)
    print()
    
    print("Picking up silver key:")
    result = await add_to_inventory("silver_key")
    print(result)
    print()
    
    print("World state after pickup:")
    state = await get_world_state()
    state_obj = json.loads(state)
    print(f"Player inventory: {state_obj['player']['inventory']}")
    print(f"Foyer items: {state_obj['locations']['foyer']['items']}")
    print()
    
    print("Trying to move north (door locked):")
    result = await move_player("study")
    print(result)
    print()
    
    print("Unlocking door with silver key:")
    result = await unlock_door("foyer", "north", "silver_key")
    print(result)
    print()
    
    print("Moving north (door now unlocked):")
    result = await move_player("study")
    print(result)
    print()
    
    print("Final world state:")
    state = await get_world_state()
    state_obj = json.loads(state)
    print(f"Player location: {state_obj['player']['location']}")
    print(f"Player inventory: {state_obj['player']['inventory']}")
    print()
    
    print("=== Test Complete ===")


if __name__ == "__main__":
    asyncio.run(test_basic_flow())