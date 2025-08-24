# Text Adventure - LLM-Directed World Mutations

## Core Guiding Principle

**The LLM is the Director of all world state changes.**

- Users express intent in natural language - any way they want
- We NEVER parse user commands directly 
- The LLM understands intent and decides what should change
- The LLM instructs specific world mutations via structured output
- World mutations are executed through MCP tools

## Architecture Philosophy

```
User Input → LLM (Director) → { Narration + World Mutations } → MCP Tools → Updated State
```

**NOT:**
```
User Input → Command Parser → Hardcoded Actions  ❌
```

## Implementation

The LLM should return structured output containing:
1. **Narration**: 2-4 sentences for the user
2. **World Mutations**: Specific MCP tool calls to execute
3. **Metadata**: Any additional context for game state

Example LLM response format:
```json
{
  "narration": "You crouch down and pick up the tarnished silver key...",
  "mutations": [
    {"tool": "add_to_inventory", "args": {"item": "silver_key"}},
    {"tool": "transfer_item", "args": {"item": "silver_key", "from_location": "foyer", "to_location": "player"}}
  ]
}
```

## Why This Approach

- **Natural language freedom**: Users can say "grab the shiny thing" or "pick up key" or "take that metal object"
- **LLM intelligence**: Leverages LLM's understanding of context and intent
- **Flexible mutations**: Can handle complex scenarios like conditional actions
- **Future-proof**: Scales to more complex world interactions
- **Consistent with vision**: Aligns with README's LLM-as-Director architecture

## Implementation Notes

- LLM has full context of current world state
- LLM decides what mutations are valid/invalid
- MCP tools provide the mutation primitives
- World state is always authoritative source of truth
- Debug logging shows both narration and executed mutations

## Observability & Tracing

### Langfuse Integration

The game integrates with Langfuse for LLM observability, performance monitoring, and session analysis.

**Setup:**
1. **Start Langfuse locally**: Navigate to `../langfuse/` and run `docker-compose up -d`
2. **Configure API keys**: Source the tracing environment with `source .env.tracing`
3. **Verify connection**: Run `./scripts/langfuse_probe.sh` to test API access

**Key Files:**
- `.env.tracing` - Contains Langfuse API credentials (update when keys change)
- `scripts/langfuse_probe.sh` - Diagnostics script for checking traces
- `internal/observability/tracer.go` - OpenTelemetry integration with Langfuse

**API Key Management:**
- Get current keys from Langfuse UI at http://localhost:3001 → Project Settings
- Update `.env.tracing` with correct `LANGFUSE_PUBLIC_KEY` and `LANGFUSE_SECRET_KEY`
- Keys are read from environment variables at runtime via `LoadConfigFromEnv()`

**Performance Analysis:**
- Use `./scripts/langfuse_probe.sh [limit]` to view recent traces
- Monitor operation timings for LLM calls (facts extraction, NPC behavior, etc.)
- All high-frequency operations use `ReasoningEffort: "minimal"` for optimal latency

## Commit Message Style

- Use one-liner format: `[tag] brief description`
- All lowercase except for the tag  
- Use descriptive tags that match the work context
- Keep messages concise and focused