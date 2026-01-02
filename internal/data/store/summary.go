package store

import (
	"time"

	"go.mau.fi/whatsmeow/types"
)

// Summary represents a conversation summary for AI context.
type Summary struct {
	ID            int64
	ChatJID       types.JID
	SummaryText   string
	TokenCount    int
	FromMessageID string
	ToMessageID   string
	CreatedAt     time.Time
}

// SummaryStore handles conversation summary operations.
type SummaryStore struct {
	store *Store
}

// NewSummaryStore creates a new SummaryStore.
func NewSummaryStore(s *Store) *SummaryStore {
	return &SummaryStore{store: s}
}

// Put stores a new summary.
func (s *SummaryStore) Put(sum *Summary) error {
	now := time.Now().Unix()
	result, err := s.store.Exec(`
		INSERT INTO orion_summaries (chat_jid, summary_text, token_count, from_message_id, to_message_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		sum.ChatJID.String(), sum.SummaryText, sum.TokenCount, sum.FromMessageID, sum.ToMessageID, now,
	)
	if err != nil {
		return err
	}
	sum.ID, _ = result.LastInsertId()
	sum.CreatedAt = time.Unix(now, 0)
	return nil
}

// GetLatest returns the most recent summary for a chat.
func (s *SummaryStore) GetLatest(chatJID types.JID) (*Summary, error) {
	row := s.store.QueryRow(`
		SELECT id, chat_jid, summary_text, token_count, from_message_id, to_message_id, created_at
		FROM orion_summaries WHERE chat_jid = ? ORDER BY created_at DESC LIMIT 1`,
		chatJID.String(),
	)

	var sum Summary
	var chatStr string
	var createdAt int64
	err := row.Scan(&sum.ID, &chatStr, &sum.SummaryText, &sum.TokenCount, &sum.FromMessageID, &sum.ToMessageID, &createdAt)
	if err != nil {
		return nil, err
	}
	sum.ChatJID, _ = types.ParseJID(chatStr)
	sum.CreatedAt = time.Unix(createdAt, 0)
	return &sum, nil
}

// GetByChatJID returns all summaries for a chat.
func (s *SummaryStore) GetByChatJID(chatJID types.JID) ([]*Summary, error) {
	rows, err := s.store.Query(`
		SELECT id, chat_jid, summary_text, token_count, from_message_id, to_message_id, created_at
		FROM orion_summaries WHERE chat_jid = ? ORDER BY created_at DESC`,
		chatJID.String(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []*Summary
	for rows.Next() {
		var sum Summary
		var chatStr string
		var createdAt int64
		if err := rows.Scan(&sum.ID, &chatStr, &sum.SummaryText, &sum.TokenCount, &sum.FromMessageID, &sum.ToMessageID, &createdAt); err != nil {
			continue
		}
		sum.ChatJID, _ = types.ParseJID(chatStr)
		sum.CreatedAt = time.Unix(createdAt, 0)
		summaries = append(summaries, &sum)
	}
	return summaries, nil
}
