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
    "textadventure/internal/llm"
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

	case npcNarrationReadyMsg:
		return m.handleNPCNarrationReady(msg)

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
		userInput := "awakening"
		m.gameHistory.AddPlayerAction(userInput)
		m.loading = true
		m.animationFrame = 0
		m.messages = append(m.messages, "LOADING_ANIMATION")
		m.turnPhase = Narration
		
        (&m).startTurn()
        ctx := m.createGameContext(m.turnContext, "director.awakening_intro")
        return m, tea.Batch(m.director.ProcessPlayerActionWithContext(ctx, userInput, m.world, m.gameHistory.GetEntries(), m.loggers.Completion), animationTimer())
    }
    return m, nil
}

func (m Model) handleNPCTurn(msg npcTurnMsg) (tea.Model, tea.Cmd) {
    if !m.loading && m.turnPhase == NPCTurns && !m.npcTurnComplete {
        m.npcTurnComplete = true
        // Enrich turn context with game/session info for NPC flows
        npcCtx := m.createGameContext(m.turnContext, "npc.turn")
        return m, actors.GenerateNPCTurn(npcCtx, m.llmService, "elena", m.world, m.gameHistory.GetEntries(), m.loggers.Debug.IsEnabled(), msg.worldEventLines)
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
            m.extractAndAccumulateFacts(m.currentResponse)
            
            m.turnPhase = PlayerTurn
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
            errorMsg := "\033[31m[ERROR] " + msg.Err.Error() + "\033[0m"
            m.messages = append(m.messages, errorMsg)
            m.gameHistory.AddError(msg.Err)
        } else {
            m.messages = append(m.messages, "\033[31m[ERROR]\033[0m "+msg.Response)
            m.gameHistory.AddNarratorResponse(msg.Response)
        }
        m.messages = append(m.messages, "")
        m.loading = false
    } else if m.streaming {
        m.streaming = false
        m.loading = false
        if msg.Err != nil {
            if len(m.messages) > 0 {
                m.messages[len(m.messages)-1] = "\033[31m[ERROR] " + msg.Err.Error() + "\033[0m"
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
		
        if msg.Debug && len(msg.WorldEventLines) > 0 {
            actorLabel := "PLAYER"
            if msg.ActingNPCID != "" {
                actorLabel = strings.ToUpper(msg.ActingNPCID)
            }
            
            header := fmt.Sprintf("\033[36m[%s WORLD EVENTS]\033[0m", actorLabel)
            m.messages = append(m.messages, header)
            for _, line := range msg.WorldEventLines {
                eventMsg := fmt.Sprintf("\033[36m  %s\033[0m", line)
                m.messages = append(m.messages, eventMsg)
            }
        }
		
        if msg.Debug && (len(msg.Mutations) > 0 || len(msg.WorldEventLines) > 0) {
            m.messages = append(m.messages, "")
        }
        
        // no accumulation needed for event lines
		
		if m.turnPhase == Narration {
			m.messages = append(m.messages, "LOADING_ANIMATION")
			
            // Narration uses world events (omniscient view) for this turn
            narrCtx := m.createGameContext(m.turnContext, "narration.generate")
            return m, narration.StartLLMStream(narrCtx, m.llmService, msg.UserInput, m.world, m.gameHistory.GetEntries(), m.loggers.Completion, m.loggers.Debug.IsEnabled(), msg.ActionContext, msg.Successes, msg.WorldEventLines, msg.ActingNPCID)
        } else {
            m.loading = false
            
            switch m.turnPhase {
            case PlayerTurn:
                m.turnPhase = NPCTurns
                m.npcTurnComplete = false
                // Compute perceptions for NPC in next step
                return m, npcTurnCmd(msg.WorldEventLines)
            case NPCTurns:
                m.turnPhase = Narration
                m.npcTurnComplete = false
                cmds := []tea.Cmd{startNarrationCmd(m.world, m.gameHistory.GetEntries(), m.loggers.Debug.IsEnabled())}
                if msg.ActingNPCID != "" {
                    cmds = append(cmds, m.generateNPCNarration(msg.ActingNPCID, msg.WorldEventLines, msg.ActionContext, msg.Successes))
                }
                return m, tea.Batch(cmds...)
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
            // Ensure spacing before the player's submitted prompt for readability
            m.messages = append(m.messages, "")
            m.messages = append(m.messages, "> "+userInput)
				switch strings.ToLower(userInput) {
				case "/worldstate", "/world", "/debug":
					worldInfo := fmt.Sprintf("[DEBUG] Current World State:")
					m.messages = append(m.messages, worldInfo)
					m.messages = append(m.messages, fmt.Sprintf("[DEBUG] Player Location: %s", m.world.Location))
					m.messages = append(m.messages, fmt.Sprintf("[DEBUG] Player Inventory: %v", m.world.Inventory))
					m.messages = append(m.messages, fmt.Sprintf("[DEBUG] Available Locations: %v", getLocationList(m.world)))
					for locID, loc := range m.world.Locations {
						m.messages = append(m.messages, fmt.Sprintf("[DEBUG] %s: %s (Facts: %v, Exits: %v)", locID, loc.Name, loc.Facts, loc.Exits))
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

// npcNarrationReadyMsg carries NPC-perspective narration back to the UI for optional display and fact extraction.
type npcNarrationReadyMsg struct {
    NPCID     string
    Narration string
}

// generateNPCNarration creates a tea.Cmd that generates a short NPC-perspective narration
// and returns it as a message. It does not affect loading/spinner states.
func (m Model) generateNPCNarration(npcID string, worldEventLines []string, actionContext string, mutationResults []string) tea.Cmd {
    return func() tea.Msg {
        worldCtx := game.BuildWorldContext(m.world, []string{}, npcID)
        systemPrompt := narration.BuildNPCNarrationPrompt(npcID, actionContext, mutationResults, worldEventLines)
        req := llm.TextCompletionRequest{
            SystemPrompt: systemPrompt,
            UserPrompt:   worldCtx + "NPC ACTION: " + strings.ToUpper(npcID),
            MaxTokens:    180,
        }
        ctx := m.createGameContext(m.sessionContext, "npc.narration")
        text, err := m.llmService.CompleteText(ctx, req)
        if err != nil {
            return npcNarrationReadyMsg{NPCID: npcID, Narration: ""}
        }
        return npcNarrationReadyMsg{NPCID: npcID, Narration: strings.TrimSpace(text)}
    }
}

func (m Model) handleNPCNarrationReady(msg npcNarrationReadyMsg) (tea.Model, tea.Cmd) {
    if msg.Narration == "" {
        return m, nil
    }
    if m.loggers.Debug.IsEnabled() {
        var colorCode string
        if npc, ok := m.world.NPCs[msg.NPCID]; ok && npc.DebugColor != "" {
            colorCode = fmt.Sprintf("\033[%sm", npc.DebugColor)
        } else {
            colorCode = "\033[36m"
        }
        header := fmt.Sprintf("%s[%s NARRATION]\033[0m", colorCode, strings.ToUpper(msg.NPCID))
        m.messages = append(m.messages, header)
        for _, line := range strings.Split(msg.Narration, "\n") {
            if s := strings.TrimSpace(line); s != "" {
                m.messages = append(m.messages, colorCode+"  "+s+"\033[0m")
            }
        }
        m.messages = append(m.messages, "")
    }
    if npc, ok := m.world.NPCs[msg.NPCID]; ok {
        m.extractAndAccumulateFactsForLocation(npc.Location, msg.Narration)
    }
    return m, nil
}

func getLocationList(world game.WorldState) []string {
	var locations []string
	for locID := range world.Locations {
		locations = append(locations, locID)
	}
	return locations
}
