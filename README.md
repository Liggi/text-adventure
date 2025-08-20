# Text Adventure - LLM-Directed World Mutations

## Comprehensive Architecture Analysis

This text adventure implements the "LLM as Director" pattern, where natural language input is understood by an LLM that orchestrates world state changes through structured mutations.

### Core Architecture

```
┌─────────────────────────────────────────────────────┐
│                   User Input                        │
└──────────────────────┬──────────────────────────────┘
                       ▼
┌─────────────────────────────────────────────────────┐
│              Bubble Tea UI (Go)                     │
│  - cmd/game/ui/{model,update,view,commands}.go     │
└──────────────────────┬──────────────────────────────┘
                       ▼
┌─────────────────────────────────────────────────────┐
│            Two-Step LLM Flow                        │
│  1. Generate Mutations (GPT-5)                      │
│  2. Execute via MCP → Narrate Results               │
└──────────┬───────────────────────────┬──────────────┘
           ▼                           ▼
┌──────────────────────┐    ┌────────────────────────┐
│   MCP Client (Go)    │    │  LLM Client (GPT-5)    │
│  internal/mcp/*      │    │  internal/llm/*        │
└──────────┬───────────┘    └────────────────────────┘
           ▼
┌──────────────────────────────────────────────────────┐
│          Python MCP Server                          │
│  services/worldstate/world_state.py                 │
│  - Authoritative world state (JSON)                 │
│  - Mutation tools (move, transfer, unlock)          │
└──────────────────────────────────────────────────────┘
```

### Design Principles

**The LLM is the Director of all world state changes:**
- Users express intent in natural language (any way they want)
- We NEVER parse user commands directly
- The LLM understands intent and decides what should change
- The LLM instructs specific world mutations via structured output
- World mutations are executed through MCP tools

**Data Flow:**
```
User Input → LLM (Director) → { Narration + World Mutations } → MCP Tools → Updated State
```

### Current File Structure

```
cmd/game/              # Application entry point
├── app.go            # App initialization with MCP setup
├── main.go           # Main entry point
└── ui/               # Bubble Tea interface
    ├── commands.go   # LLM orchestration & mutations (1000+ lines)
    ├── model.go      # UI state management
    ├── update.go     # Event handling & turn system
    └── view.go       # Terminal rendering

internal/             # Internal packages
├── debug/           # Debug logging
├── game/            # Core game types
│   ├── history.go   # Conversation history
│   └── world.go     # World state types
├── llm/             # OpenAI GPT-5 client
├── logging/         # Completion logging
└── mcp/             # MCP client & type conversion

services/            # External services
├── world_state.json # Persistent world state
└── worldstate/      # Python MCP server
    └── world_state.py # Authoritative state & mutations
```

### Key Features

1. **LLM-Directed Mutations**: GPT-5 analyzes player intent and generates specific MCP tool calls
2. **Authoritative World State**: Python MCP server maintains single source of truth
3. **Turn-Based NPCs**: NPCs react to sensory events with AI-generated thoughts and actions
4. **Sensory Event System**: Volume-based sound propagation between locations
5. **Streaming Narration**: Real-time GPT-5 streaming with context-aware responses
6. **Natural Language Freedom**: No command parsing - players can express intent however they want

### Evolution History

The project has evolved through several architectural phases:

1. **Chat Interface**: Basic Bubble Tea chat UI
2. **LLM Integration**: GPT-5 streaming with world context
3. **World State**: Static locations and items system
4. **MCP Architecture**: Python server for atomic world mutations
5. **Services Split**: Separation of UI, game logic, and world state
6. **NPC System**: Turn-based AI characters with thoughts and actions
7. **Sensory Events**: Sound propagation and NPC reactions
8. **Event Accumulation**: Player-centric narration with turn cycles

### Current Strengths

- **Natural Language Freedom**: Players can say "grab the shiny thing" or "pick up key"
- **Consistent World State**: MCP tools ensure atomic, validated mutations
- **Rich NPC Behavior**: NPCs react intelligently to sensory events
- **LLM-as-Director Pattern**: Successfully avoids command parsing
- **Sophisticated Event System**: Volume decay and multi-location sound travel

### Architectural Recommendations

The current codebase successfully implements the vision but has organizational debt:

#### Recommended File Structure (Domain-Driven)

```
game/
├── director/         # LLM orchestration
│   ├── mutations.go  # Mutation generation logic
│   ├── narration.go  # Narration with context
│   └── sensory.go    # Sensory event generation
├── world/            # World state management  
│   ├── state.go      # World state types
│   ├── sync.go       # MCP synchronization
│   └── validator.go  # Invariant validation
├── actors/           # Player & NPCs
│   ├── player.go     # Player actions
│   ├── npc.go        # NPC behavior
│   └── turns.go      # Turn management
├── ui/              # Terminal interface
└── infrastructure/
    ├── llm/         # OpenAI client
    └── mcp/         # MCP client
```

#### Key Issues to Address

1. **Monolithic commands.go**: 1000+ lines handling too many responsibilities
2. **State Sync Complexity**: Three-way sync between Python/Go/LLM representations
3. **Missing Abstractions**: No explicit Director, Turn Manager, or Event Bus
4. **Testing Gap**: No tests for critical mutation/validation logic

### Getting Started

```bash
make run      # Start the game
make debug    # Run with debug logging  
make reset    # Reset world state
make cleanlogs # Clear debug logs
```

### Architecture Philosophy

This project demonstrates that sophisticated interactive fiction can emerge from simple principles:

1. **LLM Understanding**: Let AI handle natural language complexity
2. **Structured Mutations**: Enforce consistency through typed tool calls  
3. **Event-Driven NPCs**: Create emergent behavior through sensory reactions
4. **Authoritative State**: Maintain single source of truth for world state

The result is a text adventure where players have complete natural language freedom while the world maintains consistency and NPCs exhibit intelligent behavior.