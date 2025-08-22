package debug

import (
	"log"
	"os"
)

type Logger struct {
	enabled bool
}

func NewLogger(enabled bool) *Logger {
	logFile, err := os.OpenFile("debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(logFile)
	}
	
	if enabled {
		log.Printf("=== DEBUG MODE ENABLED ===")
	} else {
		log.Printf("=== LOGGING ENABLED (UI DEBUG OFF) ===")
	}
	
	return &Logger{enabled: enabled}
}

func (d *Logger) Printf(format string, args ...interface{}) {
	log.Printf(format, args...)
}

func (d *Logger) Println(args ...interface{}) {
	log.Println(args...)
}

func (d *Logger) IsEnabled() bool {
	return d.enabled
}