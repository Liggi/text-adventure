#!/bin/bash
cd "$(dirname "$0")/.."
echo "Starting development environment..."
echo "This will start the MCP server in the background and then the game"
cd services/worldstate && export PATH="$HOME/.local/bin:$PATH" && uv run python world_state.py &
echo "MCP server started"
echo "Starting text adventure game..."
cd ../..
go run ./cmd/game