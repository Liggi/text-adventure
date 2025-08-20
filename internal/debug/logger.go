package debug

import (
	"log"
	"os"
)

type Logger struct {
	enabled bool
}

func NewLogger(enabled bool) *Logger {
	if enabled {
		logFile, err := os.OpenFile("debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			log.SetOutput(logFile)
		}
		log.Printf("=== DEBUG MODE ENABLED ===")
	}
	
	return &Logger{enabled: enabled}
}

func (d *Logger) Printf(format string, args ...interface{}) {
	if d.enabled {
		log.Printf(format, args...)
	}
}

func (d *Logger) Println(args ...interface{}) {
	if d.enabled {
		log.Println(args...)
	}
}