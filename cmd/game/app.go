package main

import (
	"fmt"
	"log"
	"os"

	"github.com/sashabaranov/go-openai"

	"textadventure/cmd/game/ui"
	"textadventure/internal/game"
	"textadventure/internal/logging"
)

func createApp() (ui.Model, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return ui.Model{}, fmt.Errorf("please set OPENAI_API_KEY environment variable")
	}
	
	client := openai.NewClient(apiKey)
	debugMode := os.Getenv("DEBUG") == "1" || os.Getenv("DEBUG") == "true"
	
	if debugMode {
		logFile, err := os.OpenFile("debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			log.SetOutput(logFile)
		}
	}
	
	logger, err := logging.NewCompletionLogger()
	if err != nil {
		return ui.Model{}, fmt.Errorf("failed to initialize completion logger: %w", err)
	}
	
	world := game.NewDefaultWorldState()
	
	model := ui.NewModel(client, world, logger, debugMode)
	
	return model, nil
}