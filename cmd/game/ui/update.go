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
		if !m.loading && m.mcpClient != nil {
			userInput := "look around"
			m.gameHistory = append(m.gameHistory, "Player: "+userInput)
			m.loading = true
			m.animationFrame = 0
			m.messages = append(m.messages, "LOADING_ANIMATION")
			m.turnPhase = Narration
			
			return m, tea.Batch(director.StartTwoStepLLMFlow(m.client, userInput, m.world, m.gameHistory, m.logger, m.mcpClient, m.debug), animationTimer())
		}
		return m, nil
		
	case npcTurnMsg:
		if !m.loading && m.turnPhase == NPCTurns && !m.npcTurnComplete {
			m.npcTurnComplete = true
			return m, actors.GenerateNPCTurn(m.client, "elena", m.world, m.gameHistory, m.debug, msg.sensoryEvents)
		}
		return m, nil
		
	case narrationTurnMsg:
		if !m.loading && m.turnPhase == Narration {
			m.loading = true
			m.animationFrame = 0
			m.messages = append(m.messages, "LOADING_ANIMATION")
			
			userInput := "narrate recent events"
			return m, tea.Batch(director.StartTwoStepLLMFlow(m.client, userInput, m.world, m.gameHistory, m.logger, m.mcpClient, m.debug), animationTimer())
		}
		return m, nil
		
	case actors.NPCThoughtsMsg:
		if msg.Debug && msg.Thoughts != "" {
			// Use the NPC's defined debug color
			var colorCode string
			if npc, exists := m.world.NPCs[msg.NPCID]; exists && npc.DebugColor != "" {
				colorCode = fmt.Sprintf("\033[%sm", npc.DebugColor)
			} else {
				colorCode = "\033[36m" // Default cyan
			}
			
			// Handle multi-line thoughts by applying color to each line
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
		
	case actors.NPCActionMsg:
		if msg.Debug && msg.Thoughts != "" {
			// Display NPC thoughts using the same logic as npcThoughtsMsg
			var colorCode string
			if npc, exists := m.world.NPCs[msg.NPCID]; exists && npc.DebugColor != "" {
				colorCode = fmt.Sprintf("\033[%sm", npc.DebugColor)
			} else {
				colorCode = "\033[36m" // Default cyan
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
		
		// If NPC has an action, execute it through the mutation pipeline
		if msg.Action != "" && !m.loading {
			if msg.Debug {
				actionMsg := fmt.Sprintf("\033[33m[%s ACTION] %s\033[0m", strings.ToUpper(msg.NPCID), msg.Action)
				m.messages = append(m.messages, actionMsg)
				m.messages = append(m.messages, "")
			}
			
			m.gameHistory = append(m.gameHistory, fmt.Sprintf("%s: %s", msg.NPCID, msg.Action))
			m.loading = true
			m.animationFrame = 0
			m.messages = append(m.messages, "LOADING_ANIMATION")
			
			return m, tea.Batch(director.StartTwoStepLLMFlow(m.client, msg.Action, m.world, m.gameHistory, m.logger, m.mcpClient, m.debug, msg.NPCID), animationTimer())
		}
		return m, nil
		
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case animationTickMsg:
		if m.loading {
			m.animationFrame++
			return m, animationTimer()
		}
		return m, nil

	case narration.StreamStartedMsg:
		if m.loading {
			m.messages = m.messages[:len(m.messages)-1]
			m.streaming = true
			m.currentResponse = ""
			m.messages = append(m.messages, "")
		}
		return m, narration.ReadNextChunk(msg.Stream, msg.Debug, &msg, "")

	case narration.StreamChunkMsg:
		if m.streaming {
			m.currentResponse += msg.Chunk
			if len(m.messages) > 0 {
				m.messages[len(m.messages)-1] = m.currentResponse
			}
		}
		return m, narration.ReadNextChunk(msg.Stream, msg.Debug, msg.CompletionCtx, m.currentResponse)

	case narration.StreamCompleteMsg:
		if m.streaming {
			m.streaming = false
			m.loading = false
			
			if len(m.messages) > 0 && m.currentResponse != "" {
				m.gameHistory = append(m.gameHistory, "Narrator: "+m.currentResponse)
				if len(m.gameHistory) > 6 {
					m.gameHistory = m.gameHistory[len(m.gameHistory)-6:]
				}
			}
			
			m.messages = append(m.messages, "")
			
			if m.turnPhase == Narration {
				m.turnPhase = PlayerTurn
				m.accumulatedSensoryEvents = []sensory.SensoryEvent{}
			}
			return m, nil
		}
		return m, nil

	case narration.StreamErrorMsg:
		if m.loading && !m.streaming {
			m.messages = m.messages[:len(m.messages)-1]
			if msg.Err != nil {
				errorMsg := "Error: " + msg.Err.Error()
				m.messages = append(m.messages, errorMsg)
				m.gameHistory = append(m.gameHistory, "Error: "+msg.Err.Error())
			} else {
				m.messages = append(m.messages, msg.Response)
				m.gameHistory = append(m.gameHistory, "Narrator: "+msg.Response)
			}
			m.messages = append(m.messages, "")
			m.loading = false
			
			if len(m.gameHistory) > 6 {
				m.gameHistory = m.gameHistory[len(m.gameHistory)-6:]
			}
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

	case director.MutationsGeneratedMsg:
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
				return m, narration.StartLLMStream(m.client, msg.UserInput, m.world, m.gameHistory, m.logger, m.debug, msg.Successes, combinedEvents, msg.ActingNPCID)
			} else {
				m.loading = false
				
				// Trigger turn phase transitions directly
				switch m.turnPhase {
				case PlayerTurn:
					m.turnPhase = NPCTurns
					m.npcTurnComplete = false
					return m, npcTurnCmd(msg.SensoryEvents)
				case NPCTurns:
					m.turnPhase = Narration
					m.npcTurnComplete = false
					return m, startNarrationCmd(m.world, m.gameHistory, m.debug)
				default:
					return m, nil
				}
			}
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "enter":
			if strings.TrimSpace(m.input) != "" && !m.loading {
				userInput := m.input
				m.input = ""
				
				// Handle debug commands
				if m.debug && strings.HasPrefix(userInput, "/") {
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
				m.gameHistory = append(m.gameHistory, "Player: "+userInput)
				m.loading = true
				m.animationFrame = 0
				m.messages = append(m.messages, "LOADING_ANIMATION")
				m.turnPhase = PlayerTurn
				
				return m, tea.Batch(director.StartTwoStepLLMFlow(m.client, userInput, m.world, m.gameHistory, m.logger, m.mcpClient, m.debug), animationTimer())
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

	return m, nil
}

func getLocationList(world game.WorldState) []string {
	var locations []string
	for locID := range world.Locations {
		locations = append(locations, locID)
	}
	return locations
}