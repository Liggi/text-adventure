package ui

import (
    "context"
    "fmt"
    "strings"
    "time"

    tea "github.com/charmbracelet/bubbletea"
    
    "textadventure/internal/game"
    "textadventure/internal/game/actors"
    "textadventure/internal/game/director"
    "textadventure/internal/game/narration"
    "textadventure/internal/game/sensory"
    "go.opentelemetry.io/otel/attribute"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case initialLookAroundMsg:
		return m.handleInitialLook(msg)
	case npcTurnMsg:
		return m.handleNPCTurn(msg)
	case narrationTurnMsg:
		return m.handleNarrationTurn(msg)

	case actors.NPCThoughtsMsg:
		return m.handleNPCThoughts(msg)
	case actors.NPCActionMsg:
		return m.handleNPCAction(msg)

	case director.MutationsGeneratedMsg:
		return m.handleMutationsGenerated(msg)

	case narration.StreamStartedMsg:
		return m.handleStreamStarted(msg)
	case narration.StreamChunkMsg:
		return m.handleStreamChunk(msg)
	case narration.StreamCompleteMsg:
		return m.handleStreamComplete(msg)
	case narration.StreamErrorMsg:
		return m.handleStreamError(msg)

	case tea.WindowSizeMsg:
		return m.handleWindowResize(msg)
	case animationTickMsg:
		return m.handleAnimation(msg)
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	}
	return m, nil
}

func (m Model) handleInitialLook(msg initialLookAroundMsg) (tea.Model, tea.Cmd) {
	if !m.loading && m.mcpClient != nil {
		userInput := "look around"
		m.gameHistory.AddPlayerAction(userInput)
		m.loading = true
		m.animationFrame = 0
		m.messages = append(m.messages, "LOADING_ANIMATION")
		m.turnPhase = Narration
		
        // Start a new turn span and context
        (&m).startTurn()
        ctx := m.createGameContext(m.turnContext, "director.initial_look")
        return m, tea.Batch(m.director.ProcessPlayerActionWithContext(ctx, userInput, m.world, m.gameHistory.GetEntries(), m.loggers.Completion), animationTimer())
    }
    return m, nil
}

func (m Model) handleNPCTurn(msg npcTurnMsg) (tea.Model, tea.Cmd) {
    if !m.loading && m.turnPhase == NPCTurns && !m.npcTurnComplete {
        m.npcTurnComplete = true
        // Enrich turn context with game/session info for NPC flows
        npcCtx := m.createGameContext(m.turnContext, "npc.turn")
        return m, actors.GenerateNPCTurn(npcCtx, m.llmService, "elena", m.world, m.gameHistory.GetEntries(), m.loggers.Debug.IsEnabled(), msg.sensoryEvents)
    }
    return m, nil
}

func (m Model) handleNarrationTurn(msg narrationTurnMsg) (tea.Model, tea.Cmd) {
	if !m.loading && m.turnPhase == Narration {
		m.loading = true
		m.animationFrame = 0
		m.messages = append(m.messages, "LOADING_ANIMATION")
		
        userInput := "narrate recent events"
        // Continue current turn context
        ctx := m.createGameContext(m.turnContext, "director.narration")
        return m, tea.Batch(m.director.ProcessPlayerActionWithContext(ctx, userInput, m.world, m.gameHistory.GetEntries(), m.loggers.Completion), animationTimer())
    }
    return m, nil
}

func (m Model) handleNPCThoughts(msg actors.NPCThoughtsMsg) (tea.Model, tea.Cmd) {
	if msg.Debug && msg.Thoughts != "" {
		var colorCode string
		if npc, exists := m.world.NPCs[msg.NPCID]; exists && npc.DebugColor != "" {
			colorCode = fmt.Sprintf("\033[%sm", npc.DebugColor)
		} else {
			colorCode = "\033[36m"
		}
		
		lines := strings.Split(msg.Thoughts, "\n")
		for i, line := range lines {
			if strings.TrimSpace(line) != "" {
				if i == 0 {
					coloredThoughts := fmt.Sprintf("%s[%s] %s\033[0m", colorCode, strings.ToUpper(msg.NPCID), line)
					m.messages = append(m.messages, coloredThoughts)
				} else {
					coloredThoughts := fmt.Sprintf("%s      %s\033[0m", colorCode, line)
					m.messages = append(m.messages, coloredThoughts)
				}
			}
		}
		m.messages = append(m.messages, "")
	}
	return m, nil
}

func (m Model) handleNPCAction(msg actors.NPCActionMsg) (tea.Model, tea.Cmd) {
	if msg.Debug && msg.Thoughts != "" {
		var colorCode string
		if npc, exists := m.world.NPCs[msg.NPCID]; exists && npc.DebugColor != "" {
			colorCode = fmt.Sprintf("\033[%sm", npc.DebugColor)
		} else {
			colorCode = "\033[36m"
		}
		
		lines := strings.Split(msg.Thoughts, "\n")
		for i, line := range lines {
			if strings.TrimSpace(line) != "" {
				if i == 0 {
					coloredThoughts := fmt.Sprintf("%s[%s] %s\033[0m", colorCode, strings.ToUpper(msg.NPCID), line)
					m.messages = append(m.messages, coloredThoughts)
				} else {
					coloredThoughts := fmt.Sprintf("%s      %s\033[0m", colorCode, line)
					m.messages = append(m.messages, coloredThoughts)
				}
			}
		}
		m.messages = append(m.messages, "")
	}
	
	if msg.Action != "" && !m.loading {
		if msg.Debug {
			actionMsg := fmt.Sprintf("\033[33m[%s ACTION] %s\033[0m", strings.ToUpper(msg.NPCID), msg.Action)
			m.messages = append(m.messages, actionMsg)
			m.messages = append(m.messages, "")
		}
		
		updateMemoryCmd := m.updateNPCMemory(msg.NPCID, msg.Thoughts, msg.Action)
		
		m.gameHistory.AddNPCAction(msg.NPCID, msg.Action)
		m.loading = true
		m.animationFrame = 0
		m.messages = append(m.messages, "LOADING_ANIMATION")
		
        // Continue current turn context
        ctx := m.createGameContext(m.turnContext, "director.npc_action")
        return m, tea.Batch(
            updateMemoryCmd,
            m.director.ProcessPlayerActionWithContext(ctx, msg.Action, m.world, m.gameHistory.GetEntries(), m.loggers.Completion, msg.NPCID), 
            animationTimer(),
        )
	}
	return m, nil
}

func (m Model) handleWindowResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	return m, nil
}

func (m Model) handleAnimation(msg animationTickMsg) (tea.Model, tea.Cmd) {
	if m.loading {
		m.animationFrame++
		return m, animationTimer()
	}
	return m, nil
}

func (m Model) handleStreamStarted(msg narration.StreamStartedMsg) (tea.Model, tea.Cmd) {
	if m.loading {
		m.messages = m.messages[:len(m.messages)-1]
		m.streaming = true
		m.currentResponse = ""
		m.messages = append(m.messages, "")
	}
	return m, narration.ReadNextChunk(msg.Stream, msg.Debug, &msg, "")
}

func (m Model) handleStreamChunk(msg narration.StreamChunkMsg) (tea.Model, tea.Cmd) {
	if m.streaming {
		m.currentResponse += msg.Chunk
		if len(m.messages) > 0 {
			m.messages[len(m.messages)-1] = m.currentResponse
		}
	}
	return m, narration.ReadNextChunk(msg.Stream, msg.Debug, msg.CompletionCtx, m.currentResponse)
}

func (m Model) handleStreamComplete(msg narration.StreamCompleteMsg) (tea.Model, tea.Cmd) {
    if m.streaming {
        m.streaming = false
        m.loading = false
        
        if len(m.messages) > 0 && m.currentResponse != "" {
            m.gameHistory.AddNarratorResponse(m.currentResponse)
        }
        
        m.messages = append(m.messages, "")

        // Finalize narration span if present
        if msg.Span != nil {
            duration := time.Since(msg.StartTime)
            msg.Span.SetAttributes(
                attribute.String("langfuse.observation.output", m.currentResponse),
                attribute.Int64("response_time_ms", duration.Milliseconds()),
            )
            msg.Span.End()
        }

        if m.turnPhase == Narration {
            m.turnPhase = PlayerTurn
            m.accumulatedSensoryEvents = []sensory.SensoryEvent{}
            // End the turn span as narration completes the turn
            (&m).endTurn("narration_complete")
        }
        return m, nil
    }
    return m, nil
}

func (m Model) handleStreamError(msg narration.StreamErrorMsg) (tea.Model, tea.Cmd) {
	if m.loading && !m.streaming {
		m.messages = m.messages[:len(m.messages)-1]
		if msg.Err != nil {
			errorMsg := "Error: " + msg.Err.Error()
			m.messages = append(m.messages, errorMsg)
			m.gameHistory.AddError(msg.Err)
		} else {
			m.messages = append(m.messages, msg.Response)
			m.gameHistory.AddNarratorResponse(msg.Response)
		}
		m.messages = append(m.messages, "")
		m.loading = false
	} else if m.streaming {
		m.streaming = false
		m.loading = false
		if msg.Err != nil {
			if len(m.messages) > 0 {
				m.messages[len(m.messages)-1] = "Error: " + msg.Err.Error()
			}
			m.messages = append(m.messages, "")
		}
	}
	return m, nil
}

func (m Model) handleMutationsGenerated(msg director.MutationsGeneratedMsg) (tea.Model, tea.Cmd) {
	if m.loading {
		m.messages = m.messages[:len(m.messages)-1]
		m.world = msg.NewWorld
		
		if msg.Debug && len(msg.Mutations) > 0 {
			actorLabel := "PLAYER"
			if msg.ActingNPCID != "" {
				actorLabel = strings.ToUpper(msg.ActingNPCID)
			}
			
			mutationHeader := fmt.Sprintf("\033[35m[%s MUTATIONS]\033[0m", actorLabel)
			m.messages = append(m.messages, mutationHeader)
			
			for _, mutation := range msg.Mutations {
				if !strings.HasPrefix(mutation, "[MUTATIONS]") {
					coloredMutation := fmt.Sprintf("\033[35m  %s\033[0m", mutation)
					m.messages = append(m.messages, coloredMutation)
				}
			}
		}
		
		if len(msg.Failures) > 0 && msg.Debug {
			for _, failure := range msg.Failures {
				coloredError := fmt.Sprintf("\033[31m  [ERROR] %s\033[0m", failure)
				m.messages = append(m.messages, coloredError)
			}
		}
		
		if msg.Debug && msg.SensoryEvents != nil {
			actorLabel := "PLAYER"
			if msg.ActingNPCID != "" {
				actorLabel = strings.ToUpper(msg.ActingNPCID)
			}
			
			if len(msg.SensoryEvents.AuditoryEvents) > 0 {
				sensoryHeader := fmt.Sprintf("\033[36m[%s SENSORY EVENTS]\033[0m", actorLabel)
				m.messages = append(m.messages, sensoryHeader)
				for _, event := range msg.SensoryEvents.AuditoryEvents {
					eventMsg := fmt.Sprintf("\033[36m  🔊 %s (%s) at %s\033[0m", event.Description, event.Volume, event.Location)
					m.messages = append(m.messages, eventMsg)
				}
			} else {
				sensoryHeader := fmt.Sprintf("\033[36m[%s SENSORY EVENTS] No auditory events\033[0m", actorLabel)
				m.messages = append(m.messages, sensoryHeader)
			}
		}
		
		if msg.Debug && (len(msg.Mutations) > 0 || msg.SensoryEvents != nil) {
			m.messages = append(m.messages, "")
		}
		
		if msg.SensoryEvents != nil {
			m.accumulatedSensoryEvents = append(m.accumulatedSensoryEvents, msg.SensoryEvents.AuditoryEvents...)
		}
		
		if m.turnPhase == Narration {
			m.messages = append(m.messages, "LOADING_ANIMATION")
			
			// Filter sensory events for narrator to prevent duplication with action context
			var narratorSensoryEvents []sensory.SensoryEvent
			if msg.ActingNPCID != "" {
				narratorSensoryEvents = m.accumulatedSensoryEvents
			} else {
				// Player acted: exclude sensory events from player's location to avoid "PLAYER: Hello" + "someone shouted Hello"
				if m.loggers.Debug.IsEnabled() {
					m.loggers.Debug.Printf("Filtering sensory events for narrator. Player location: %s", m.world.Location)
					m.loggers.Debug.Printf("Total accumulated events: %d", len(m.accumulatedSensoryEvents))
					for i, event := range m.accumulatedSensoryEvents {
						m.loggers.Debug.Printf("  Event %d: %s at %s (excluding: %v)", i, event.Description, event.Location, event.Location == m.world.Location)
					}
				}
				for _, event := range m.accumulatedSensoryEvents {
					if event.Location != m.world.Location {
						narratorSensoryEvents = append(narratorSensoryEvents, event)
					}
				}
				if m.loggers.Debug.IsEnabled() {
					m.loggers.Debug.Printf("Events passed to narrator: %d", len(narratorSensoryEvents))
				}
			}
			
            combinedEvents := &sensory.SensoryEventResponse{AuditoryEvents: narratorSensoryEvents}
            narrCtx := m.createGameContext(m.turnContext, "narration.generate")
            return m, narration.StartLLMStream(narrCtx, m.llmService, msg.UserInput, m.world, m.gameHistory.GetEntries(), m.loggers.Completion, m.loggers.Debug.IsEnabled(), msg.ActionContext, msg.Successes, combinedEvents, msg.ActingNPCID)
		} else {
			m.loading = false
			
			switch m.turnPhase {
			case PlayerTurn:
				m.turnPhase = NPCTurns
				m.npcTurnComplete = false
				return m, npcTurnCmd(msg.SensoryEvents)
			case NPCTurns:
				m.turnPhase = Narration
				m.npcTurnComplete = false
				return m, startNarrationCmd(m.world, m.gameHistory.GetEntries(), m.loggers.Debug.IsEnabled())
			default:
				return m, nil
			}
		}
	}
	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "enter":
		if strings.TrimSpace(m.input) != "" && !m.loading {
			userInput := m.input
			m.input = ""
			
			if m.loggers.Debug.IsEnabled() && strings.HasPrefix(userInput, "/") {
				m.messages = append(m.messages, "> "+userInput)
				switch strings.ToLower(userInput) {
				case "/worldstate", "/world", "/debug":
					worldInfo := fmt.Sprintf("[DEBUG] Current World State:")
					m.messages = append(m.messages, worldInfo)
					m.messages = append(m.messages, fmt.Sprintf("[DEBUG] Player Location: %s", m.world.Location))
					m.messages = append(m.messages, fmt.Sprintf("[DEBUG] Player Inventory: %v", m.world.Inventory))
					m.messages = append(m.messages, fmt.Sprintf("[DEBUG] Available Locations: %v", getLocationList(m.world)))
					for locID, loc := range m.world.Locations {
						m.messages = append(m.messages, fmt.Sprintf("[DEBUG] %s: %s (Items: %v, Exits: %v)", locID, loc.Title, loc.Items, loc.Exits))
					}
				case "/help":
					m.messages = append(m.messages, "[DEBUG] Available commands:")
					m.messages = append(m.messages, "[DEBUG] /worldstate - Show current world state")
					m.messages = append(m.messages, "[DEBUG] /help - Show this help")
				default:
					m.messages = append(m.messages, "[DEBUG] Unknown command. Try /help")
				}
				m.messages = append(m.messages, "")
				return m, nil
			}
			
			m.messages = append(m.messages, "> "+userInput)
			m.messages = append(m.messages, "")
			m.gameHistory.AddPlayerAction(userInput)
			m.loading = true
			m.animationFrame = 0
			m.messages = append(m.messages, "LOADING_ANIMATION")
			m.turnPhase = PlayerTurn
			
            // Start a new turn span and context
            (&m).startTurn()
            ctx := m.createGameContext(m.turnContext, "director.player_input")
            return m, tea.Batch(m.director.ProcessPlayerActionWithContext(ctx, userInput, m.world, m.gameHistory.GetEntries(), m.loggers.Completion), animationTimer())
        }
        return m, nil

	case "backspace":
		if len(m.input) > 0 && !m.loading {
			m.input = m.input[:len(m.input)-1]
		}
		return m, nil

	default:
		if len(msg.String()) == 1 && !m.loading {
			m.input += msg.String()
		}
		return m, nil
	}
}

func (m Model) updateNPCMemory(npcID, thoughts, action string) tea.Cmd {
	return func() tea.Msg {
		if m.mcpClient == nil {
			return nil
		}
		
		ctx := context.Background()
		_, err := m.mcpClient.UpdateNPCMemory(ctx, npcID, thoughts, action)
		if err != nil && m.loggers.Debug.IsEnabled() {
			m.loggers.Debug.Printf("Failed to update NPC memory for %s: %v", npcID, err)
		}
		
		return nil
	}
}

func getLocationList(world game.WorldState) []string {
	var locations []string
	for locID := range world.Locations {
		locations = append(locations, locID)
	}
	return locations
}
