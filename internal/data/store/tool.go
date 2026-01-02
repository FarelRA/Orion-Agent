package store

import (
	"database/sql"
	"time"
)

// ToolRecord represents a tool call/result record.
type ToolRecord struct {
	MessageID   string
	ChatJID     string
	ToolCalls   []byte // JSON array
	ToolResults []byte // JSON array
	CreatedAt   int64
}

// ToolStore handles persistence of AI tool calls.
type ToolStore struct {
	db *sql.DB
}

// NewToolStore creates a new ToolStore.
func NewToolStore(db *sql.DB) *ToolStore {
	return &ToolStore{db: db}
}

// Put saves tool calls and results for a message.
func (s *ToolStore) Put(messageID, chatJID string, toolCalls, toolResults []byte) error {
	query := `
		INSERT INTO orion_tools (message_id, chat_jid, tool_calls, tool_results, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(message_id) DO UPDATE SET
			tool_calls = excluded.tool_calls,
			tool_results = excluded.tool_results`

	_, err := s.db.Exec(query, messageID, chatJID, toolCalls, toolResults, time.Now().Unix())
	return err
}

// GetByMessageID retrieves tool record by message ID.
func (s *ToolStore) GetByMessageID(messageID string) (*ToolRecord, error) {
	query := `SELECT message_id, chat_jid, tool_calls, tool_results, created_at
		FROM orion_tools WHERE message_id = ?`

	var record ToolRecord
	err := s.db.QueryRow(query, messageID).Scan(
		&record.MessageID,
		&record.ChatJID,
		&record.ToolCalls,
		&record.ToolResults,
		&record.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// GetByChatJID retrieves all tool records for a chat.
func (s *ToolStore) GetByChatJID(chatJID string) ([]*ToolRecord, error) {
	query := `SELECT message_id, chat_jid, tool_calls, tool_results, created_at
		FROM orion_tools WHERE chat_jid = ? ORDER BY created_at DESC`

	rows, err := s.db.Query(query, chatJID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*ToolRecord
	for rows.Next() {
		var record ToolRecord
		err := rows.Scan(
			&record.MessageID,
			&record.ChatJID,
			&record.ToolCalls,
			&record.ToolResults,
			&record.CreatedAt,
		)
		if err != nil {
			continue
		}
		records = append(records, &record)
	}
	return records, nil
}
