#!/bin/bash
# Run the text adventure game
cd "$(dirname "$0")/.."
go run cmd/game/main.go cmd/game/completions.go