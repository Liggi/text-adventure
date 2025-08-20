package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	
	"textadventure/internal/game"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case initialLookAroundMsg:
		if !m.loading && m.mcpClient != nil {
			userInput := "look around"
			// Don't add the user input to messages - keep it invisible
			m.gameHistory = append(m.gameHistory, "Player: "+userInput)
			m.loading = true
			m.animationFrame = 0
			m.messages = append(m.messages, "LOADING_ANIMATION")
			
			return m, tea.Batch(startTwoStepLLMFlow(m.client, userInput, m.world, m.gameHistory, m.logger, m.mcpClient, m.debug), animationTimer())
		}
		return m, nil
		
	case npcTurnMsg:
		if !m.loading {
			return m, generateNPCThoughts(m.client, "elena", m.world, m.gameHistory, m.debug)
		}
		return m, nil
		
	case npcThoughtsMsg:
		if msg.debug && msg.thoughts != "" {
			// Use the NPC's defined debug color
			var colorCode string
			if npc, exists := m.world.NPCs[msg.npcID]; exists && npc.DebugColor != "" {
				colorCode = fmt.Sprintf("\033[%sm", npc.DebugColor)
			} else {
				colorCode = "\033[36m" // Default cyan
			}
			
			// Handle multi-line thoughts by applying color to each line
			lines := strings.Split(msg.thoughts, "\n")
			for i, line := range lines {
				if strings.TrimSpace(line) != "" {
					if i == 0 {
						coloredThoughts := fmt.Sprintf("%s[%s] %s\033[0m", colorCode, strings.ToUpper(msg.npcID), line)
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

	case streamStartedMsg:
		if m.loading {
			m.messages = m.messages[:len(m.messages)-1]
			m.streaming = true
			m.currentResponse = ""
			m.messages = append(m.messages, "")
		}
		return m, readNextChunk(msg.stream, msg.debug, &msg, "")

	case llmStreamChunkMsg:
		if m.streaming {
			m.currentResponse += msg.chunk
			if len(m.messages) > 0 {
				m.messages[len(m.messages)-1] = m.currentResponse
			}
		}
		return m, readNextChunk(msg.stream, msg.debug, msg.completionCtx, m.currentResponse)

	case llmStreamCompleteMsg:
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
			
			// Trigger NPC turn after player turn completes
			return m, npcTurnCmd()
		}
		return m, nil

	case llmResponseMsg:
		if m.loading && !m.streaming {
			m.messages = m.messages[:len(m.messages)-1]
			if msg.err != nil {
				errorMsg := "Error: " + msg.err.Error()
				m.messages = append(m.messages, errorMsg)
				m.gameHistory = append(m.gameHistory, "Error: "+msg.err.Error())
			} else {
				m.messages = append(m.messages, msg.response)
				m.gameHistory = append(m.gameHistory, "Narrator: "+msg.response)
			}
			m.messages = append(m.messages, "")
			m.loading = false
			
			if len(m.gameHistory) > 6 {
				m.gameHistory = m.gameHistory[len(m.gameHistory)-6:]
			}
		} else if m.streaming {
			m.streaming = false
			m.loading = false
			if msg.err != nil {
				if len(m.messages) > 0 {
					m.messages[len(m.messages)-1] = "Error: " + msg.err.Error()
				}
				m.messages = append(m.messages, "")
			}
		}
		return m, nil

	case mutationsGeneratedMsg:
		if m.loading {
			m.messages = m.messages[:len(m.messages)-1]
			m.world = msg.newWorld
			
			if msg.debug && len(msg.mutations) > 0 {
				for _, mutation := range msg.mutations {
					m.messages = append(m.messages, mutation)
				}
			}
			
			if len(msg.failures) > 0 && msg.debug {
				for _, failure := range msg.failures {
					m.messages = append(m.messages, "[ERROR] "+failure)
				}
			}
			
			m.messages = append(m.messages, "LOADING_ANIMATION")
			
			return m, startLLMStream(m.client, msg.userInput, m.world, m.gameHistory, m.logger, m.debug, msg.successes)
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
				
				// Normal game input
				m.messages = append(m.messages, "> "+userInput)
				m.messages = append(m.messages, "")
				m.gameHistory = append(m.gameHistory, "Player: "+userInput)
				m.loading = true
				m.animationFrame = 0
				m.messages = append(m.messages, "LOADING_ANIMATION")
				
				return m, tea.Batch(startTwoStepLLMFlow(m.client, userInput, m.world, m.gameHistory, m.logger, m.mcpClient, m.debug), animationTimer())
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