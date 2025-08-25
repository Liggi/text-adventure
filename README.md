An experimental LLM-powered text adventure that uses AI as the game director to interpret natural language commands and orchestrate complex interactions between the player and NPCs. Features a sophisticated multi-prompt pipeline for realistic character behavior, world state management, and immersive storytelling.

## 🎮 Collaborative LLM / Player storytelling 

**LLM-as-Director Architecture**: Instead of parsing commands like traditional text adventures, this game uses large language models to understand player intent and decide what should change in the world.

**Natural Language Freedom**: Players can express actions any way they want:
- `"grab the shiny thing"` instead of rigid `"take key"`
- `"shout for Elena"` instead of `"say Hello Elena"`
- `"examine the dusty tiles"` for detailed environment exploration

**Dynamic NPC Behavior**: Each NPC has:
- Multi-layered perception system (what they can actually see/hear)
- Internal thought generation based on personality and memories
- Contextual action decisions influenced by their backstory
- Realistic limitations on what they know about the world


## 🏗️ Architecture Overview

```
User Input → Director (LLM) → World Mutations (MCP) → Event Generation → 
NPC Perception → NPC Thoughts → NPC Actions → Narration → Return to Player
```

### Core Components

- **Director (`internal/game/director/`)**: Central LLM orchestrator that interprets intent and coordinates world changes
- **MCP Integration**: External Python world state service provides authoritative state management
- **NPC System (`internal/game/actors/`)**: Multi-prompt character behavior with perception, thoughts, and actions
- **Narration (`internal/game/narration/`)**: Story-focused LLM that presents results to the player

## 🚀 Getting Started

### Prerequisites

- **Go 1.21+** for the main game engine
- **Python 3.8+** for the MCP world state server
- **OpenAI API key** for LLM calls

### Installation

1. **Clone the repository**:
   ```bash
   git clone <repo-url>
   cd text-adventure
   ```

2. **Run the setup script**:
   ```bash
   chmod +x setup.sh
   ./setup.sh
   ```
   
   This will check your system, install dependencies, and build the game.

3. **Set your OpenAI API key**:
   ```bash
   export OPENAI_API_KEY='your-api-key-here'
   ```

4. **Start playing**:
   ```bash
   make          # Start the game
   make debug    # Start with debug logging
   make reset    # Reset game state manually
   ```

## 🔧 MCP Integration

The game implements the **Model Context Protocol (MCP)** for world state management, providing a clean separation between game logic and state storage.

### Why MCP?

- **Future LLM Integration**: The MCP server could eventually allow LLMs to directly manipulate world state
- **Seeding & Testing**: Easy to programmatically set up world scenarios
- **State Persistence**: World state can be saved/restored independently of game sessions
- **Multi-Client Support**: Multiple game clients could theoretically connect to the same world

### MCP Tools Available

The game exposes these MCP tools for world manipulation:

- `get_world_state()` - Retrieve current world snapshot
- `move_player(location)` - Change player location
- `move_npc(npc_id, location)` - Move an NPC
- `transfer_item(item, from_location, to_location)` - Move items between locations/inventories
- `add_to_inventory(item)` / `remove_from_inventory(item)` - Inventory management
- `mark_npc_as_met(npc_id)` - Track social interactions

## 🎯 Playing the Game

### Basic Commands

The beauty of this system is that you don't need to learn specific commands. Just express what you want to do naturally:

- **Movement**: `"go to the library"`, `"head north"`, `"walk into the kitchen"`
- **Interaction**: `"talk to Elena"`, `"ask about the journal"`, `"examine the desk"`
- **Actions**: `"pick up the key"`, `"put the book on the table"`, `"open the door"`
- **Communication**: `"shout for help"`, `"whisper 'hello'"`, `"call out Elena's name"`

### Understanding NPCs

NPCs in this game have realistic limitations:
- They only know what they can see, hear, or remember
- Their actions are influenced by personality, backstory, and recent experiences
- They form thoughts before taking actions, creating believable behavior
- They can be in different locations and won't know about events they can't perceive

## 🛠️ Development

### Project Structure

```
internal/
├── game/
│   ├── director/          # LLM intent interpretation & world mutations
│   ├── actors/            # NPC behavior system
│   ├── narration/         # Story presentation
│   ├── perception/        # NPC sensory filtering
│   └── sensory/          # Environmental event generation
├── llm/                  # OpenAI integration & tracing
├── mcp/                  # Model Context Protocol client
└── observability/        # Langfuse tracing setup
```
