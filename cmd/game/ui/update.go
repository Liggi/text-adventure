package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "enter":
			if strings.TrimSpace(m.input) != "" && !m.loading {
				userInput := m.input
				m.messages = append(m.messages, "> "+userInput)
				m.messages = append(m.messages, "")
				m.gameHistory = append(m.gameHistory, "Player: "+userInput)
				m.input = ""
				m.loading = true
				m.animationFrame = 0
				m.messages = append(m.messages, "LOADING_ANIMATION")
				
				return m, tea.Batch(startLLMStream(m.client, userInput, m.world, m.gameHistory, m.logger, m.debug), animationTimer())
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