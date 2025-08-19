# Text Adventure — First Room Plan (LLM-Directed)

This plan outlines how to stand up the very first playable “room” using an LLM-directed runtime. It adapts the architecture you described (LLM as Director + JSON Patch world edits) to this Go + Bubble Tea project.

## Goals
- Minimal, shippable loop: type input → see narration → world updates.
- One starting location with a clear affordance path (look, take, open).
- Human-readable `WorldDoc` as the single source of truth.
- LLM outputs concise narration + a minimal JSON Patch.
- Lightweight validator guards invariants (no teleport, stable IDs, etc.).
- Turn-by-turn persistence for rewind/debugging.

## Deliverables (MVP)
- `world/seed.yaml` (or `.json`): initial `WorldDoc` with one location, one item, one door.
- `internal/world` package: load/save doc, apply JSON Patch, validate invariants.
- `internal/director` package: prompt assembly, LLM call, output parsing.
- `internal/persist` package: append-only JSONL or SQLite turns (pick one).
- TUI loop integration (existing Bubble Tea) calling the Director and rendering narration.

## WorldDoc Shape (seed)
Use YAML (human-friendly) or JSON. Keep IDs stable; separate canonical facts from mutable state.

```yaml
world:
  locations:
    - id: foyer
      title: Old Foyer
      canonical_facts:
        - a locked oak door leads north to the study
        - dust motes drift in a shaft of light
      mutable_state:
        door_locked: true
      exits:
        north: study
      entities_here: [silver_key]
    - id: study
      title: Quiet Study
      canonical_facts:
        - a heavy oak desk dominates the room
      mutable_state: {}
      exits:
        south: foyer
      entities_here: []
  entities:
    - id: silver_key
      kind: item
      affordances: [take, drop, use_on:door]
  npcs: []
  player:
    location: foyer
    inventory: []
  clocks:
    - id: nightfall
      ticks_remaining: 8
meta:
  style: "classic interactive fiction, concise"
  invariants:
    - ids are stable; do not repurpose
    - do not contradict canonical_facts without explicit retcon
    - player cannot move through locked doors without cause
  frontier: ["study mystery"]
```

## LLM Output Contract
Ask the Director to return narration + JSON Patch (RFC 6902) + optional open hooks.

```json
{
  "narration": "2–5 sentence vivid description...",
  "world_patch": [
    {"op":"replace","path":"/player/location","value":"study"},
    {"op":"replace","path":"/world/locations/0/mutable_state/door_locked","value":false},
    {"op":"add","path":"/meta/frontier/-","value":"strange ledger in desk"}
  ],
  "open_hooks": ["unlock desk", "read ledger"]
}
```

## Step-by-Step Plan
1) Define seed world
- Create `world/seed.yaml` from the example above (YAML or JSON).
- Commit to stable IDs (`foyer`, `study`, `silver_key`).

2) World package
- `internal/world/doc.go`: structs mirroring the seed (locations, entities, player, meta).
- `internal/world/load.go`: load/save YAML/JSON; normalize maps/slices for stable JSON pointer paths.
- `internal/world/patch.go`: apply RFC 6902 JSON Patch (e.g., `github.com/evanphx/json-patch/v5`).
- `internal/world/validate.go`: invariants check (stable IDs, door restrictions, non-negative clocks, exits consistent both ways if declared).

3) Director package
- `internal/director/prompt.go`: build system + checklist prompt using:
  - Current `WorldDoc`, last 2–3 turns transcript, new player input.
  - Checklist: “Advance clocks if warranted; don’t spawn items; keep exits consistent; suggest visible affordances on failure.”
- `internal/director/call.go`: call OpenAI client (already present) and parse JSON payload.
- `internal/director/schema.go`: strict JSON schema validation for `narration`, `world_patch`, `open_hooks`.

4) Critic/validator pass (optional but recommended)
- If `world_patch` violates invariants, either:
  - Bounce and re-ask the Director with a short diagnostic; or
  - Run a “critic mode” prompt that proposes the smallest corrected patch.

5) Persistence
- Start simple: append JSONL `data/turns.jsonl` with `{turn_index, world_doc, patch, narration, ts}`.
- Or use SQLite (`github.com/mattn/go-sqlite3`): table `turns(id INTEGER PK, ts, world_doc TEXT, patch TEXT, narration TEXT)`.
- Add `persist.LoadLatest()` and `persist.SaveTurn()` helpers.

6) TUI integration
- On startup: load latest world or `seed.yaml`; print opening narration (from `seed` or a scripted “look”).
- On Enter: send input + transcript tail to Director; get `narration` + `world_patch`.
- Validate; apply patch; persist; stream narration to the chat panel.
- If invalid, show a brief in-world failure narration (from Director) and keep state unchanged.

7) First-room acceptance tests (manual)
- `look` in foyer returns description with exits and visible items.
- `take key` moves `silver_key` to inventory; removes from foyer.
- `open door` fails if locked; succeeds after using key; moves player to study on `north`.
- `inventory` lists `silver_key` when held.
- Clocks never go negative; IDs remain stable across turns; exits remain coherent.

8) Prompts (initial draft)
- System: “You are both narrator and world simulator. Produce 2–5 sentence narration, a minimal JSON Patch to update the provided WorldDoc, and optional open hooks. Keep IDs stable. Obey invariants. If action is impossible, narrate failure and suggest visible affordances.”
- Checklist appended to the user message: “Advance clocks only with cause; don’t spawn items; keep exits consistent; no teleport without on-screen reason; keep narration concise.”

9) Logging & debug
- Log raw Director I/O, applied patches, and validator messages in `debug.log` (redact secrets).
- Add a `--replay` mode to print the transcript and world deltas for debugging.

10) Nice-to-have next
- Status pane (location, exits, inventory, clocks) alongside narration.
- “GM screen” hidden notes in `meta` that are not shown to player.
- Swap JSONL → SQLite once flow feels right.

## Ready-to-Build Checklist
- WorldDoc seed file exists and loads.
- Director returns valid JSON Patch for trivial actions (`look`, `take`, `open`).
- Validator blocks illegal state transitions (e.g., walking through locked door).
- Persistence captures each turn (doc + patch + narration).
- TUI shows narration and handles streaming gracefully.

Once these are green, you’ll have a solid first room you can grow entirely by playing—no hand-written reducers required.

