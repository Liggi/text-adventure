# Text Adventure - Event System Analysis

## Current System Architecture

This text adventure implements an LLM-as-Director architecture with a sophisticated event system. Here's how it currently works and where improvements are needed.

### Current Game Flow
```
1. PlayerTurn: User input → Director → Mutations → Sensory Events
2. NPCTurns: Each NPC → Thoughts → Actions → Director → Mutations → Sensory Events  
3. Narration: All accumulated sensory events → LLM Narrator → Player story
```

### Current Event System Architecture

#### **Sensory Event Generation**
- **Location**: `internal/game/sensory/events.go`
- **Purpose**: Generate auditory events from actions and mutations
- **LLM Prompt**: Forces everything through "sounds you can hear" lens
- **Output**: JSON with `type`, `description`, `location`, `volume`

```go
type SensoryEvent struct {
    Type        string `json:"type"`        // Currently only "auditory"
    Description string `json:"description"` // "someone walked from foyer to library"
    Location    string `json:"location"`    // Where the sound occurs
    Volume      string `json:"volume"`      // "quiet", "moderate", "loud"
}
```

#### **Volume Decay System**
- **Location**: `internal/game/sensory/events.go` (CalculateRoomDistance, ApplyVolumeDecay)
- **Purpose**: Sound travels between rooms with realistic distance decay
- **Implementation**: BFS pathfinding + volume reduction by distance

#### **Narration Integration** 
- **Location**: `internal/game/narration/prompts.go`
- **Input**: All accumulated sensory events from the turn
- **Process**: Narrator weaves events into coherent story
- **Problem**: Narrator only knows "what sound occurred where" - no spatial context

## Identified Problems

### 1. **Temporal Confusion**
**The Issue**: Events are generated **after** mutations complete, but narrator describes them as if happening **during** the action.

**Example**: 
- Elena moves from foyer → library
- System generates: `"footsteps approached and crossed a threshold" at library`  
- Narrator says: *"Elena's footsteps approach from the east and cross the threshold into the library"*
- **Wrong**: Elena was moving **TO** the east, not **FROM** the east

### 2. **Spatial Ambiguity**
**The Issue**: Sensory events only have `location` field, no directional context.

- `"footsteps at library"` could mean arriving OR departing
- No `from_location` field to indicate movement direction
- Narrator must guess spatial relationships from limited data

### 3. **Limited Event Types**
**The Issue**: Everything forced through "auditory" lens loses important context.

- Movement becomes "footsteps"
- Speech becomes "someone said X"  
- Interactions become generic "sounds"
- Loss of actor identity and spatial relationships

### 4. **Narrator Overreach**
**The Issue**: Narrator must infer complex spatial relationships from post-hoc sound descriptions.

- No access to mutation details ("Elena moved from foyer to library")  
- Only gets processed sound events ("footsteps at library")
- Creates inconsistencies when guessing directions and relationships

## Potential Solutions

### **Option A: Enhance Sensory Events**
Add directional context to existing system:

```go
type SensoryEvent struct {
    Type         string `json:"type"`
    Description  string `json:"description"`
    Location     string `json:"location"`
    FromLocation string `json:"from_location,omitempty"` // For movements
    Volume       string `json:"volume,omitempty"`
    Actor        string `json:"actor,omitempty"`         // Who did the action
}
```

### **Option B: Separate Event System**
Replace sensory events with broader world events:

```go
type WorldEvent struct {
    Type     string                 // "movement", "speech", "interaction", "sound"
    Actor    string                 // "player", "elena", etc.
    Action   string                 // "walked", "said", "picked up"
    Location string 
    Target   string                 // destination, item, etc.
    Details  map[string]interface{} // flexible context
}
```

### **Option C: Give Narrator Direct Access to Mutations**
Simplest fix - pass mutations directly to narrator:

- Narrator sees: `"Elena moved from foyer to library"`
- Plus sensory events for atmospheric details
- Clear spatial context, no guessing required

### **Option D: Structured Action Events**
Comprehensive event system with typed actions:

```go
type ActionEvent struct {
    Actor          string         `json:"actor"`
    Action         string         `json:"action"` // "move", "say", "take"
    FromLocation   string         `json:"from_location,omitempty"`
    ToLocation     string         `json:"to_location,omitempty"`
    Target         string         `json:"target,omitempty"` // item, person
    Content        string         `json:"content,omitempty"` // spoken words
    SensoryEffects []SensoryEvent `json:"sensory_effects"`
}
```

## Questions for Direction

1. **Scope**: Fix just movement events, or redesign the whole event system?

2. **Narrator Intelligence**: Should narrator have direct access to mutations, or stay purely event-driven?

3. **Event Granularity**: Single flexible event type vs. specialized types for different actions?

4. **Backwards Compatibility**: Do we need to maintain the current sensory event API?

## Recommended Next Steps

**Option C** (narrator gets mutations) seems like the simplest fix that would solve the immediate directional confusion while preserving the existing sensory system for atmospheric details.

**Option D** (structured action events) would be the most robust long-term solution, creating a comprehensive event system that could support rich NPC reasoning and complex interactions.

The current NPC personality system (with backstory, core memories, and personality traits) is working well and would integrate cleanly with any of these event system improvements.

## Proposed Architecture: Multi-Layered Event System

### Event Flow & Ontology Diagram

#### **Current State (Problematic)**
```
Player Input → Director → Mutations → Sensory Events → Everyone
                                         ↓
                                   (Lost Context)
                                         ↓
                              NPCs + Narrator (Confused)
```

#### **Proposed: Multi-Layered Event System**

```
┌─────────────────────────────────────────────────────────────────┐
│                        DIRECTOR (Global Truth)                  │
└─────────────────────────┬───────────────────────────────────────┘
                          │
                    World Events
                 (Canonical Reality)
                          │
        ┌─────────────────┼─────────────────┐
        ▼                 ▼                 ▼
   
┌─────────────┐  ┌─────────────────┐  ┌─────────────────┐
│   NARRATOR  │  │  META-NARRATOR  │  │  NPC PERCEPTION │
│             │  │                 │  │                 │
│ • Full      │  │ • System-level  │  │ • Filtered      │
│   Context   │  │   Events        │  │   Sensory       │
│ • Who/What  │  │ • Time Passage  │  │ • Distance      │
│ • Results   │  │ • State Changes │  │ • Realistic     │
│ • Immersion │  │ • Transitions   │  │   Limits        │
└─────────────┘  └─────────────────┘  └─────────────────┘
        │                 │                     │
        ▼                 ▼                     ▼
   
┌─────────────┐  ┌─────────────────┐  ┌─────────────────┐
│   PLAYER    │  │   SYSTEM UI     │  │   NPC REACTIONS │
│  IMMERSION  │  │                 │  │                 │
│             │  │ • Turn Changes  │  │ • Thoughts      │
│ • Story     │  │ • Phase Shifts  │  │ • Actions       │
│ • Reactions │  │ • Debug Info    │  │ • Memory        │
│ • Atmosphere│  │ • State Updates │  │ • Dialogue      │
└─────────────┘  └─────────────────┘  └─────────────────┘
```

### Event Ontology

#### **1. World Events (Canonical)**
```go
type WorldEvent struct {
    ID        string
    Actor     ActorID    // "player", "elena", "system"
    Action    ActionType // "say", "move", "take", "unlock" 
    Target    string     // item, location, person
    Content   string     // spoken words, item name
    Location  string     // where it occurred
    Success   bool       // did it work?
    Timestamp time.Time
    Metadata  map[string]interface{}
}
```

#### **2. Perception Events (NPC-Filtered)**
```go
type PerceptionEvent struct {
    Observer  string     // which NPC
    Type      string     // "auditory", "visual", "tactile"
    Content   string     // what they perceived
    Location  string     // where they perceived it
    Volume    VolumeType // sound intensity
    Certainty float64    // how sure they are
}
```

#### **3. Meta Events (System-Level)**
```go
type MetaEvent struct {
    Type      string     // "turn_start", "turn_end", "phase_change"
    Context   string     // additional info
    Actor     string     // who triggered it
    Timestamp time.Time
}
```

#### **4. Narration Events (Story-Level)**
```go
type NarrationEvent struct {
    WorldEvent   *WorldEvent
    Consequences []string    // what happened as a result
    Atmosphere   string      // mood/setting changes
    Focus        string      // what to emphasize
}
```

### Event Flow Examples

#### **Example 1: Player Says "Hello?"**
```
World Event: {Actor: "player", Action: "say", Content: "Hello?", Location: "foyer"}
    │
    ├─→ Narrator: Gets full context, describes consequences only
    │   Output: "A voice answers from the library: 'Hold there—I'm coming.'"
    │
    ├─→ Elena (Library): {Type: "auditory", Content: "voice called 'Hello?'", Volume: "moderate"}
    │   Reaction: Decides to respond based on personality/memories
    │
    └─→ Meta-Narrator: {Type: "dialogue_initiated", Context: "player-elena"}
        UI: Could show turn phase changes, debug info
```

#### **Example 2: Elena Takes Journal**
```
World Event: {Actor: "elena", Action: "take", Target: "journal", Location: "library"}
    │
    ├─→ Narrator: "Gets full context about Elena's action"
    │   Output: Describes consequences for player's awareness
    │
    ├─→ Player (Foyer): {Type: "auditory", Content: "rustling, scraping", Volume: "quiet"}
    │   Experience: Hears mysterious sounds from library
    │
    └─→ Meta-Narrator: {Type: "world_state_change", Context: "item_moved"}
```

### System Responsibilities

#### **Director**
- Generates canonical World Events
- Maintains authoritative game state
- Distributes events to appropriate systems

#### **Narrator** 
- Receives World Events with full context
- Focuses on consequences and reactions
- Never re-describes player actions
- Creates immersive story experience

#### **Meta-Narrator**
- Handles system-level storytelling
- Time passage, phase transitions
- Environmental changes
- UI state management

#### **Perception Filter**
- Converts World Events → Perception Events per NPC
- Applies realistic limitations (distance, obstacles)
- Maintains consistency in what NPCs can know

### Benefits of This Architecture

1. **Eliminates Echo Problem**: Narrator knows player spoke "Hello?" vs. hearing someone else say it
2. **Consistent NPC Knowledge**: NPCs only react to what they can realistically perceive
3. **Rich Storytelling**: Multiple narrative layers for different aspects of the experience
4. **Scalable**: Easy to add new event types, NPCs, or interaction patterns
5. **Debuggable**: Clear separation between what happened vs. who knows what

This architecture would completely solve issues like the "Hello?" echo while creating a robust foundation for complex multi-NPC interactions and rich storytelling.

## Observability & Debugging Strategy

### Current Debugging Challenges

The complexity of our LLM-driven architecture creates significant debugging difficulties:

- **Multiple LLM calls per turn**: Director → Sensory Events → NPC Thoughts → NPC Actions → Director → Sensory Events → Narrator (up to 8 calls per turn)
- **Complex state transitions**: PlayerTurn → NPCTurns → Narration phases with accumulated events
- **Context passing**: Information flows between systems with filtering and transformations
- **Hard to trace causality**: When something goes wrong, difficult to identify which LLM call or context caused the issue

### Recommended Solution: Langfuse Integration

**Langfuse** is an open-source LLM observability platform designed for complex multi-step workflows like ours.

#### What It Provides

**Visual Turn Timeline**
```
Turn #5: Player says "Hello?"
├─ Director LLM (120ms, $0.003) → "no mutations needed"
├─ Sensory Events LLM (200ms, $0.002) → "voice called out..."  
├─ Elena Thoughts LLM (180ms, $0.004) → "someone's here, hide items"
├─ Elena Action LLM (150ms, $0.003) → "take crystal_orb"
├─ Director LLM (140ms, $0.003) → "Elena took orb"
├─ Sensory Events LLM (190ms, $0.002) → "fabric rustling..."
└─ Narrator LLM (250ms, $0.005) → Final story
```

**Input/Output Inspection**
- Click any LLM call to see exact prompts and responses
- View all context passed to each system
- Track token usage and costs per call
- Compare before/after when making changes

**Performance Analytics**
- Cost per turn and total session cost
- Latency bottlenecks (which calls take longest)
- Token usage patterns and optimization opportunities
- Error correlation across the pipeline

#### Debugging Workflow Improvements

**Problem**: "Elena's dialogue isn't being quoted"
- **Current**: Scan logs, guess which narrator call, manually trace context
- **With Langfuse**: Click narrator call → see exact sensory events → test prompt variations in playground

**Problem**: "Player sensory events reaching narrator"  
- **Current**: Add debug prints, rebuild, test, remove prints
- **With Langfuse**: Search player speech turns → inspect narrator input → trace filtering logic

**Problem**: "NPCs making weird decisions"
- **Current**: Correlate NPC thoughts with world state across multiple log entries
- **With Langfuse**: View Elena's decision timeline → see exact context for each decision → identify patterns

#### Implementation Options

**Go Support**: Langfuse supports Go applications through OpenTelemetry integration (OpenLLMetry/OpenLIT) or direct API access.

**Integration Points**:
- Trace each LLM service call with context
- Track turn phases and state transitions  
- Log world state changes and mutations
- Correlate player actions with NPC reactions

#### Expected Benefits

1. **Faster debugging**: Transform "hunt through logs" to "click and inspect"
2. **Cost optimization**: Identify expensive or redundant LLM calls
3. **Quality monitoring**: Track conversation quality and repetition patterns
4. **Architecture validation**: Measure impact of event system changes
5. **Performance tuning**: Find and fix latency bottlenecks

This observability foundation would support both current debugging needs and future architectural improvements like the multi-layered event system.