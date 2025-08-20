package logging

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type CompletionLog struct {
	ID           int       `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	WorldState   string    `json:"world_state"`
	UserInput    string    `json:"user_input"`
	SystemPrompt string    `json:"system_prompt"`
	Response     string    `json:"response"`
	Metadata     string    `json:"metadata"`
}

type CompletionMetadata struct {
	Model           string        `json:"model"`
	MaxTokens       int           `json:"max_tokens"`
	ResponseTime    time.Duration `json:"response_time_ms"`
	StreamingUsed   bool          `json:"streaming_used"`
	Error           *string       `json:"error,omitempty"`
}

type CompletionLogger struct {
	db *sql.DB
}

func NewCompletionLogger() (*CompletionLogger, error) {
	db, err := sql.Open("sqlite3", "./completions.db")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	logger := &CompletionLogger{db: db}
	if err := logger.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return logger, nil
}

func (cl *CompletionLogger) createTables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS completions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		world_state TEXT NOT NULL,
		user_input TEXT NOT NULL,
		system_prompt TEXT NOT NULL,
		response TEXT NOT NULL,
		metadata TEXT NOT NULL
	);
	
	CREATE INDEX IF NOT EXISTS idx_completions_timestamp ON completions(timestamp);
	`

	_, err := cl.db.Exec(schema)
	return err
}

func (cl *CompletionLogger) LogCompletion(
	worldState interface{},
	userInput string,
	systemPrompt string,
	response string,
	metadata CompletionMetadata,
) error {
	worldStateJson, err := json.Marshal(worldState)
	if err != nil {
		return fmt.Errorf("failed to marshal world state: %w", err)
	}

	metadataJson, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	_, err = cl.db.Exec(`
		INSERT INTO completions (world_state, user_input, system_prompt, response, metadata)
		VALUES (?, ?, ?, ?, ?)
	`, string(worldStateJson), userInput, systemPrompt, response, string(metadataJson))

	return err
}

func (cl *CompletionLogger) Close() error {
	return cl.db.Close()
}