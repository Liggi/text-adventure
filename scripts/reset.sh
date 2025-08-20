#!/bin/bash

PROJECT_ROOT="$(dirname "$0")/.."
cd "$PROJECT_ROOT"

echo "Resetting world state..."

# Remove the persistent world state file
if [ -f "services/world_state.json" ]; then
    rm "services/world_state.json"
    echo "✓ Removed existing world state file"
else
    echo "✓ No existing world state file found"
fi

# Also clear debug logs if they exist
if [ -f "debug.log" ]; then
    > debug.log
    echo "✓ Cleared debug log"
fi

echo ""
echo "🎮 World state has been reset!"
echo "   Next game start will use the default world configuration"
echo "   - Player will be in the foyer"  
echo "   - All items will be in their starting locations"
echo "   - All doors will be in their default locked/unlocked state"