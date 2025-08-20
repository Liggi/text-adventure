#!/bin/bash
cd "$(dirname "$0")/.."
go build -o textadventure cmd/game/main.go cmd/game/completions.go