package main

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
	Rating       *int      `json:"rating,omitempty"`
	Notes        *string   `json:"notes,omitempty"`
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
		metadata TEXT NOT NULL,
		rating INTEGER,
		notes TEXT
	);
	
	CREATE INDEX IF NOT EXISTS idx_completions_timestamp ON completions(timestamp);
	CREATE INDEX IF NOT EXISTS idx_completions_rating ON completions(rating);
	`

	_, err := cl.db.Exec(schema)
	return err
}

func (cl *CompletionLogger) LogCompletion(
	worldState WorldState,
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

func (cl *CompletionLogger) GetRecentCompletions(limit int) ([]CompletionLog, error) {
	rows, err := cl.db.Query(`
		SELECT id, timestamp, world_state, user_input, system_prompt, response, metadata, rating, notes
		FROM completions 
		ORDER BY timestamp DESC 
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var completions []CompletionLog
	for rows.Next() {
		var c CompletionLog
		err := rows.Scan(&c.ID, &c.Timestamp, &c.WorldState, &c.UserInput, 
			&c.SystemPrompt, &c.Response, &c.Metadata, &c.Rating, &c.Notes)
		if err != nil {
			return nil, err
		}
		completions = append(completions, c)
	}

	return completions, rows.Err()
}

func (cl *CompletionLogger) RateCompletion(id int, rating int, notes string) error {
	var notesPtr *string
	if notes != "" {
		notesPtr = &notes
	}

	_, err := cl.db.Exec(`
		UPDATE completions 
		SET rating = ?, notes = ? 
		WHERE id = ?
	`, rating, notesPtr, id)

	return err
}

func (cl *CompletionLogger) Close() error {
	return cl.db.Close()
}