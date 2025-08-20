#!/usr/bin/make -f

.DEFAULT_GOAL := run

run:
	@go run main.go completions.go

review:
	@go run main.go completions.go review

rate:
	@echo "Usage: make rate ID=123 RATING=4 NOTES=\"good response\""
	@if [ -z "$(ID)" ] || [ -z "$(RATING)" ]; then exit 1; fi
	@go run main.go completions.go rate $(ID) $(RATING) $(NOTES)

build:
	@go build -o text-adventure main.go completions.go

clean:
	@rm -f text-adventure completions.db debug.log

# Provide a stable target without hyphen and alias the hyphenated one.
.PHONY: mcp_server mcp-server

mcp_server:
	@echo "Starting MCP server for world state mutations..."
	@cd mcp-server && export PATH="$$HOME/.local/bin:$$PATH" && uv run python world_state.py

# Backwards-compatible alias so `make mcp-server` works everywhere
mcp-server: mcp_server

dev:
	@echo "Starting development environment..."
	@echo "This will start the MCP server in the background and then the game"
	@cd mcp-server && export PATH="$$HOME/.local/bin:$$PATH" && uv run python world_state.py &
	@echo "MCP server started"
	@echo "Starting text adventure game..."
	@go run main.go completions.go

.PHONY: run review rate build clean dev
