package store

import (
	"time"

	"go.mau.fi/whatsmeow/types"
)

// Reaction represents a message reaction.
type Reaction struct {
	MessageID string
	ChatJID   types.JID
	SenderLID types.JID
	Emoji     string
	Timestamp time.Time
}

// ReactionStore handles reaction operations.
type ReactionStore struct {
	store *Store
}

// NewReactionStore creates a new ReactionStore.
func NewReactionStore(s *Store) *ReactionStore {
	return &ReactionStore{store: s}
}

// Put saves or updates a reaction (upsert).
func (s *ReactionStore) Put(r *Reaction) error {
	_, err := s.store.Exec(`
		INSERT INTO orion_reactions (message_id, chat_jid, sender_lid, emoji, timestamp)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(message_id, chat_jid, sender_lid) DO UPDATE SET
			emoji = excluded.emoji,
			timestamp = excluded.timestamp
	`, r.MessageID, r.ChatJID.String(), r.SenderLID.String(), r.Emoji, r.Timestamp.Unix())
	return err
}

// Delete removes a reaction.
func (s *ReactionStore) Delete(messageID string, chatJID, senderLID types.JID) error {
	_, err := s.store.Exec(`
		DELETE FROM orion_reactions WHERE message_id = ? AND chat_jid = ? AND sender_lid = ?
	`, messageID, chatJID.String(), senderLID.String())
	return err
}

// GetByMessage returns all reactions for a message.
func (s *ReactionStore) GetByMessage(messageID string, chatJID types.JID) ([]Reaction, error) {
	rows, err := s.store.Query(`
		SELECT message_id, chat_jid, sender_lid, emoji, timestamp
		FROM orion_reactions
		WHERE message_id = ? AND chat_jid = ?
		ORDER BY timestamp DESC
	`, messageID, chatJID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reactions []Reaction
	for rows.Next() {
		var r Reaction
		var chatJIDStr, senderLIDStr string
		var ts int64

		if err := rows.Scan(&r.MessageID, &chatJIDStr, &senderLIDStr, &r.Emoji, &ts); err != nil {
			return nil, err
		}

		r.ChatJID, _ = types.ParseJID(chatJIDStr)
		r.SenderLID, _ = types.ParseJID(senderLIDStr)
		r.Timestamp = time.Unix(ts, 0)

		reactions = append(reactions, r)
	}

	return reactions, nil
}

// GetBySender returns all reactions by a specific sender.
func (s *ReactionStore) GetBySender(senderLID types.JID) ([]Reaction, error) {
	rows, err := s.store.Query(`
		SELECT message_id, chat_jid, sender_lid, emoji, timestamp
		FROM orion_reactions
		WHERE sender_lid = ?
		ORDER BY timestamp DESC
	`, senderLID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reactions []Reaction
	for rows.Next() {
		var r Reaction
		var chatJIDStr, senderLIDStr string
		var ts int64

		if err := rows.Scan(&r.MessageID, &chatJIDStr, &senderLIDStr, &r.Emoji, &ts); err != nil {
			return nil, err
		}

		r.ChatJID, _ = types.ParseJID(chatJIDStr)
		r.SenderLID, _ = types.ParseJID(senderLIDStr)
		r.Timestamp = time.Unix(ts, 0)

		reactions = append(reactions, r)
	}

	return reactions, nil
}
