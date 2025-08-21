package director

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sashabaranov/go-openai"

	"textadventure/internal/debug"
	"textadventure/internal/game"
	"textadventure/internal/logging"
	"textadventure/internal/mcp"
)

func StartTwoStepLLMFlow(client *openai.Client, userInput string, world game.WorldState, gameHistory []string, logger *logging.CompletionLogger, mcpClient *mcp.WorldStateClient, debugLogger *debug.Logger, actingNPCID ...string) tea.Cmd {
	director := NewDirector(client, mcpClient, debugLogger)
	return director.ProcessPlayerAction(userInput, world, gameHistory, logger, actingNPCID...)
}