#!/bin/bash
PROJECT_ROOT="$(dirname "$0")/.."
cd "$PROJECT_ROOT"

echo "Starting development environment in DEBUG MODE..."
echo "Debug logs will be written to debug.log"
echo "This will start the MCP server in the background and then the game"

> debug.log

cd services/worldstate && export PATH="$HOME/.local/bin:$PATH" && uv run python world_state.py &
echo "MCP server started"

echo "Starting text adventure game in DEBUG MODE..."
cd "$PROJECT_ROOT"

DEBUG=1 go run ./cmd/game