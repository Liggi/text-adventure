#!/bin/bash
cd "$(dirname "$0")/.."
echo "Starting MCP server for world state mutations..."
cd services/worldstate && export PATH="$HOME/.local/bin:$PATH" && uv run python world_state.py