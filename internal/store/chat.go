package store

import (
	"database/sql"
	"time"

	"go.mau.fi/whatsmeow/types"
)

// ChatType represents the type of chat.
type ChatType string

const (
	ChatTypeUser       ChatType = "user"
	ChatTypeGroup      ChatType = "group"
	ChatTypeNewsletter ChatType = "newsletter"
	ChatTypeBroadcast  ChatType = "broadcast"
	ChatTypeStatus     ChatType = "status"
)

// Chat represents a complete chat with all conversation state.
type Chat struct {
	JID      types.JID
	ChatType ChatType
	Name     string

	// Message counts
	UnreadCount        int
	UnreadMentionCount int
	LastMessageAt      time.Time
	LastMessageID      string

	// State
	IsArchived     bool
	IsPinned       bool
	PinTimestamp   time.Time
	MutedUntil     time.Time
	MarkedAsUnread bool

	// Ephemeral
	EphemeralDuration         uint32
	EphemeralSettingTimestamp time.Time

	// Sync
	ConversationTimestamp time.Time
	EndOfHistoryTransfer  bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

// ChatStore handles chat operations.
type ChatStore struct {
	store *Store
}

// NewChatStore creates a new ChatStore.
func NewChatStore(s *Store) *ChatStore {
	return &ChatStore{store: s}
}

// Put stores or updates a chat.
func (s *ChatStore) Put(c *Chat) error {
	now := time.Now().Unix()
	var lastMsgAt, pinTs, mutedUntil, ephemeralSettingTs, convTs sql.NullInt64

	if !c.LastMessageAt.IsZero() {
		lastMsgAt.Int64 = c.LastMessageAt.Unix()
		lastMsgAt.Valid = true
	}
	if !c.PinTimestamp.IsZero() {
		pinTs.Int64 = c.PinTimestamp.Unix()
		pinTs.Valid = true
	}
	if !c.MutedUntil.IsZero() {
		mutedUntil.Int64 = c.MutedUntil.Unix()
		mutedUntil.Valid = true
	}
	if !c.EphemeralSettingTimestamp.IsZero() {
		ephemeralSettingTs.Int64 = c.EphemeralSettingTimestamp.Unix()
		ephemeralSettingTs.Valid = true
	}
	if !c.ConversationTimestamp.IsZero() {
		convTs.Int64 = c.ConversationTimestamp.Unix()
		convTs.Valid = true
	}

	_, err := s.store.Exec(`
		INSERT INTO orion_chats (
			jid, chat_type, name,
			unread_count, unread_mention_count, last_message_at, last_message_id,
			is_archived, is_pinned, pin_timestamp, muted_until, marked_as_unread,
			ephemeral_duration, ephemeral_setting_timestamp,
			conversation_timestamp, end_of_history_transfer,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(jid) DO UPDATE SET
			name = COALESCE(excluded.name, orion_chats.name),
			unread_count = excluded.unread_count,
			unread_mention_count = excluded.unread_mention_count,
			last_message_at = COALESCE(excluded.last_message_at, orion_chats.last_message_at),
			last_message_id = COALESCE(excluded.last_message_id, orion_chats.last_message_id),
			is_archived = excluded.is_archived,
			is_pinned = excluded.is_pinned,
			pin_timestamp = COALESCE(excluded.pin_timestamp, orion_chats.pin_timestamp),
			muted_until = COALESCE(excluded.muted_until, orion_chats.muted_until),
			marked_as_unread = excluded.marked_as_unread,
			ephemeral_duration = COALESCE(excluded.ephemeral_duration, orion_chats.ephemeral_duration),
			ephemeral_setting_timestamp = COALESCE(excluded.ephemeral_setting_timestamp, orion_chats.ephemeral_setting_timestamp),
			conversation_timestamp = COALESCE(excluded.conversation_timestamp, orion_chats.conversation_timestamp),
			end_of_history_transfer = excluded.end_of_history_transfer,
			updated_at = excluded.updated_at
	`, c.JID.String(), string(c.ChatType), nullString(c.Name),
		c.UnreadCount, c.UnreadMentionCount, lastMsgAt, nullString(c.LastMessageID),
		boolToInt(c.IsArchived), boolToInt(c.IsPinned), pinTs, mutedUntil, boolToInt(c.MarkedAsUnread),
		int(c.EphemeralDuration), ephemeralSettingTs,
		convTs, boolToInt(c.EndOfHistoryTransfer),
		now, now)
	return err
}

// Get retrieves a chat by JID.
func (s *ChatStore) Get(jid types.JID) (*Chat, error) {
	row := s.store.QueryRow(`
		SELECT jid, chat_type, name,
			unread_count, unread_mention_count, last_message_at, last_message_id,
			is_archived, is_pinned, pin_timestamp, muted_until, marked_as_unread,
			ephemeral_duration, ephemeral_setting_timestamp,
			conversation_timestamp, end_of_history_transfer,
			created_at, updated_at
		FROM orion_chats WHERE jid = ?
	`, jid.String())

	return s.scanChat(row)
}

// GetAll retrieves all chats ordered by last message.
func (s *ChatStore) GetAll() ([]*Chat, error) {
	rows, err := s.store.Query(`
		SELECT jid, chat_type, name,
			unread_count, unread_mention_count, last_message_at, last_message_id,
			is_archived, is_pinned, pin_timestamp, muted_until, marked_as_unread,
			ephemeral_duration, ephemeral_setting_timestamp,
			conversation_timestamp, end_of_history_transfer,
			created_at, updated_at
		FROM orion_chats
		ORDER BY is_pinned DESC, last_message_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []*Chat
	for rows.Next() {
		c, err := s.scanChatRow(rows)
		if err != nil {
			return nil, err
		}
		chats = append(chats, c)
	}
	return chats, nil
}

// EnsureExists creates a chat if it doesn't exist.
func (s *ChatStore) EnsureExists(jid types.JID, chatType ChatType) error {
	now := time.Now().Unix()
	_, err := s.store.Exec(`
		INSERT INTO orion_chats (jid, chat_type, unread_count, unread_mention_count, created_at, updated_at)
		VALUES (?, ?, 0, 0, ?, ?)
		ON CONFLICT(jid) DO NOTHING
	`, jid.String(), string(chatType), now, now)
	return err
}

// Exists checks if a chat exists.
func (s *ChatStore) Exists(jid types.JID) (bool, error) {
	var count int
	err := s.store.QueryRow(`SELECT COUNT(*) FROM orion_chats WHERE jid = ?`, jid.String()).Scan(&count)
	return count > 0, err
}

// SetArchived updates archive status.
func (s *ChatStore) SetArchived(jid types.JID, archived bool) error {
	now := time.Now().Unix()
	_, err := s.store.Exec(`
		UPDATE orion_chats SET is_archived = ?, updated_at = ? WHERE jid = ?
	`, boolToInt(archived), now, jid.String())
	return err
}

// SetPinned updates pin status.
func (s *ChatStore) SetPinned(jid types.JID, pinned bool, timestamp time.Time) error {
	now := time.Now().Unix()
	var pinTs sql.NullInt64
	if pinned && !timestamp.IsZero() {
		pinTs.Int64 = timestamp.Unix()
		pinTs.Valid = true
	}
	_, err := s.store.Exec(`
		UPDATE orion_chats SET is_pinned = ?, pin_timestamp = ?, updated_at = ? WHERE jid = ?
	`, boolToInt(pinned), pinTs, now, jid.String())
	return err
}

// SetMuted updates mute status.
func (s *ChatStore) SetMuted(jid types.JID, mutedUntil time.Time) error {
	now := time.Now().Unix()
	var muted sql.NullInt64
	if !mutedUntil.IsZero() {
		muted.Int64 = mutedUntil.Unix()
		muted.Valid = true
	}
	_, err := s.store.Exec(`
		UPDATE orion_chats SET muted_until = ?, updated_at = ? WHERE jid = ?
	`, muted, now, jid.String())
	return err
}

// SetMarkedAsUnread updates marked as unread status.
func (s *ChatStore) SetMarkedAsUnread(jid types.JID, marked bool) error {
	now := time.Now().Unix()
	_, err := s.store.Exec(`
		UPDATE orion_chats SET marked_as_unread = ?, updated_at = ? WHERE jid = ?
	`, boolToInt(marked), now, jid.String())
	return err
}

// IncrementUnread increments unread count.
func (s *ChatStore) IncrementUnread(jid types.JID, hasMention bool) error {
	now := time.Now().Unix()
	mentionIncr := 0
	if hasMention {
		mentionIncr = 1
	}
	_, err := s.store.Exec(`
		UPDATE orion_chats SET 
			unread_count = unread_count + 1, 
			unread_mention_count = unread_mention_count + ?,
			updated_at = ? 
		WHERE jid = ?
	`, mentionIncr, now, jid.String())
	return err
}

// MarkRead resets unread count.
func (s *ChatStore) MarkRead(jid types.JID) error {
	now := time.Now().Unix()
	_, err := s.store.Exec(`
		UPDATE orion_chats SET unread_count = 0, unread_mention_count = 0, marked_as_unread = 0, updated_at = ? WHERE jid = ?
	`, now, jid.String())
	return err
}

// UpdateLastMessage updates the last message info.
func (s *ChatStore) UpdateLastMessage(jid types.JID, messageID string, timestamp time.Time) error {
	now := time.Now().Unix()
	_, err := s.store.Exec(`
		UPDATE orion_chats SET last_message_id = ?, last_message_at = ?, updated_at = ? WHERE jid = ?
	`, messageID, timestamp.Unix(), now, jid.String())
	return err
}

// SetEphemeral updates ephemeral settings.
func (s *ChatStore) SetEphemeral(jid types.JID, duration uint32, timestamp time.Time) error {
	now := time.Now().Unix()
	var ts sql.NullInt64
	if !timestamp.IsZero() {
		ts.Int64 = timestamp.Unix()
		ts.Valid = true
	}
	_, err := s.store.Exec(`
		UPDATE orion_chats SET ephemeral_duration = ?, ephemeral_setting_timestamp = ?, updated_at = ? WHERE jid = ?
	`, int(duration), ts, now, jid.String())
	return err
}

// Delete deletes a chat.
func (s *ChatStore) Delete(jid types.JID) error {
	_, err := s.store.Exec(`DELETE FROM orion_chats WHERE jid = ?`, jid.String())
	return err
}

// Clear clears all messages but keeps the chat.
func (s *ChatStore) Clear(jid types.JID) error {
	now := time.Now().Unix()
	_, err := s.store.Exec(`
		UPDATE orion_chats SET unread_count = 0, unread_mention_count = 0, updated_at = ? WHERE jid = ?
	`, now, jid.String())
	if err != nil {
		return err
	}
	// Also delete messages
	_, err = s.store.Exec(`DELETE FROM orion_messages WHERE chat_jid = ?`, jid.String())
	return err
}

func (s *ChatStore) scanChat(row *sql.Row) (*Chat, error) {
	var jidStr, chatType string
	var name, lastMsgID sql.NullString
	var unreadCount, unreadMentionCount int
	var lastMsgAt, pinTs, mutedUntil, ephemeralSettingTs, convTs sql.NullInt64
	var ephemeralDur int
	var isArchived, isPinned, markedAsUnread, endOfHistory int
	var createdAt, updatedAt int64

	err := row.Scan(
		&jidStr, &chatType, &name,
		&unreadCount, &unreadMentionCount, &lastMsgAt, &lastMsgID,
		&isArchived, &isPinned, &pinTs, &mutedUntil, &markedAsUnread,
		&ephemeralDur, &ephemeralSettingTs,
		&convTs, &endOfHistory,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	jid, _ := types.ParseJID(jidStr)

	c := &Chat{
		JID:                  jid,
		ChatType:             ChatType(chatType),
		Name:                 name.String,
		UnreadCount:          unreadCount,
		UnreadMentionCount:   unreadMentionCount,
		LastMessageID:        lastMsgID.String,
		IsArchived:           isArchived == 1,
		IsPinned:             isPinned == 1,
		MarkedAsUnread:       markedAsUnread == 1,
		EphemeralDuration:    uint32(ephemeralDur),
		EndOfHistoryTransfer: endOfHistory == 1,
		CreatedAt:            time.Unix(createdAt, 0),
		UpdatedAt:            time.Unix(updatedAt, 0),
	}

	if lastMsgAt.Valid {
		c.LastMessageAt = time.Unix(lastMsgAt.Int64, 0)
	}
	if pinTs.Valid {
		c.PinTimestamp = time.Unix(pinTs.Int64, 0)
	}
	if mutedUntil.Valid {
		c.MutedUntil = time.Unix(mutedUntil.Int64, 0)
	}
	if ephemeralSettingTs.Valid {
		c.EphemeralSettingTimestamp = time.Unix(ephemeralSettingTs.Int64, 0)
	}
	if convTs.Valid {
		c.ConversationTimestamp = time.Unix(convTs.Int64, 0)
	}

	return c, nil
}

func (s *ChatStore) scanChatRow(rows *sql.Rows) (*Chat, error) {
	var jidStr, chatType string
	var name, lastMsgID sql.NullString
	var unreadCount, unreadMentionCount int
	var lastMsgAt, pinTs, mutedUntil, ephemeralSettingTs, convTs sql.NullInt64
	var ephemeralDur int
	var isArchived, isPinned, markedAsUnread, endOfHistory int
	var createdAt, updatedAt int64

	err := rows.Scan(
		&jidStr, &chatType, &name,
		&unreadCount, &unreadMentionCount, &lastMsgAt, &lastMsgID,
		&isArchived, &isPinned, &pinTs, &mutedUntil, &markedAsUnread,
		&ephemeralDur, &ephemeralSettingTs,
		&convTs, &endOfHistory,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	jid, _ := types.ParseJID(jidStr)

	c := &Chat{
		JID:                  jid,
		ChatType:             ChatType(chatType),
		Name:                 name.String,
		UnreadCount:          unreadCount,
		UnreadMentionCount:   unreadMentionCount,
		LastMessageID:        lastMsgID.String,
		IsArchived:           isArchived == 1,
		IsPinned:             isPinned == 1,
		MarkedAsUnread:       markedAsUnread == 1,
		EphemeralDuration:    uint32(ephemeralDur),
		EndOfHistoryTransfer: endOfHistory == 1,
		CreatedAt:            time.Unix(createdAt, 0),
		UpdatedAt:            time.Unix(updatedAt, 0),
	}

	if lastMsgAt.Valid {
		c.LastMessageAt = time.Unix(lastMsgAt.Int64, 0)
	}
	if pinTs.Valid {
		c.PinTimestamp = time.Unix(pinTs.Int64, 0)
	}
	if mutedUntil.Valid {
		c.MutedUntil = time.Unix(mutedUntil.Int64, 0)
	}
	if ephemeralSettingTs.Valid {
		c.EphemeralSettingTimestamp = time.Unix(ephemeralSettingTs.Int64, 0)
	}
	if convTs.Valid {
		c.ConversationTimestamp = time.Unix(convTs.Int64, 0)
	}

	return c, nil
}
