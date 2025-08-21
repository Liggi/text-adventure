package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	
	"textadventure/internal/game"
	"textadventure/internal/game/actors"
	"textadventure/internal/game/director"
	"textadventure/internal/game/narration"
	"textadventure/internal/game/sensory"
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
		
		return m, tea.Batch(m.director.ProcessPlayerAction(userInput, m.world, m.gameHistory.GetEntries(), m.loggers.Completion), animationTimer())
	}
	return m, nil
}

func (m Model) handleNPCTurn(msg npcTurnMsg) (tea.Model, tea.Cmd) {
	if !m.loading && m.turnPhase == NPCTurns && !m.npcTurnComplete {
		m.npcTurnComplete = true
		return m, actors.GenerateNPCTurn(m.client, "elena", m.world, m.gameHistory.GetEntries(), m.loggers.Debug.IsEnabled(), msg.sensoryEvents)
	}
	return m, nil
}

func (m Model) handleNarrationTurn(msg narrationTurnMsg) (tea.Model, tea.Cmd) {
	if !m.loading && m.turnPhase == Narration {
		m.loading = true
		m.animationFrame = 0
		m.messages = append(m.messages, "LOADING_ANIMATION")
		
		userInput := "narrate recent events"
		return m, tea.Batch(m.director.ProcessPlayerAction(userInput, m.world, m.gameHistory.GetEntries(), m.loggers.Completion), animationTimer())
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
		
		m.gameHistory.AddNPCAction(msg.NPCID, msg.Action)
		m.loading = true
		m.animationFrame = 0
		m.messages = append(m.messages, "LOADING_ANIMATION")
		
		return m, tea.Batch(m.director.ProcessPlayerAction(msg.Action, m.world, m.gameHistory.GetEntries(), m.loggers.Completion, msg.NPCID), animationTimer())
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
		
		if m.turnPhase == Narration {
			m.turnPhase = PlayerTurn
			m.accumulatedSensoryEvents = []sensory.SensoryEvent{}
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
			for _, mutation := range msg.Mutations {
				m.messages = append(m.messages, mutation)
			}
		}
		
		if len(msg.Failures) > 0 && msg.Debug {
			for _, failure := range msg.Failures {
				m.messages = append(m.messages, "[ERROR] "+failure)
			}
		}
		
		if msg.Debug && msg.SensoryEvents != nil {
			if len(msg.SensoryEvents.AuditoryEvents) > 0 {
				m.messages = append(m.messages, "[SENSORY EVENTS]")
				for _, event := range msg.SensoryEvents.AuditoryEvents {
					eventMsg := fmt.Sprintf("  🔊 %s (%s) at %s", event.Description, event.Volume, event.Location)
					m.messages = append(m.messages, eventMsg)
				}
			} else {
				m.messages = append(m.messages, "[SENSORY EVENTS] No auditory events")
			}
		}
		
		if msg.SensoryEvents != nil {
			m.accumulatedSensoryEvents = append(m.accumulatedSensoryEvents, msg.SensoryEvents.AuditoryEvents...)
		}
		
		if m.turnPhase == Narration {
			m.messages = append(m.messages, "LOADING_ANIMATION")
			
			combinedEvents := &sensory.SensoryEventResponse{AuditoryEvents: m.accumulatedSensoryEvents}
			return m, narration.StartLLMStream(m.client, msg.UserInput, m.world, m.gameHistory.GetEntries(), m.loggers.Completion, m.loggers.Debug.IsEnabled(), msg.Successes, combinedEvents, msg.ActingNPCID)
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
			
			return m, tea.Batch(m.director.ProcessPlayerAction(userInput, m.world, m.gameHistory.GetEntries(), m.loggers.Completion), animationTimer())
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

func getLocationList(world game.WorldState) []string {
	var locations []string
	for locID := range world.Locations {
		locations = append(locations, locID)
	}
	return locations
}