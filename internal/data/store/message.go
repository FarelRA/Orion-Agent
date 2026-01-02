package store

import (
	"database/sql"
	"encoding/json"
	"time"

	"go.mau.fi/whatsmeow/types"
)

// Message represents a complete message with all fields for media download.
type Message struct {
	// Identity
	ID        string
	ChatJID   types.JID
	SenderLID types.JID
	FromMe    bool
	Timestamp time.Time
	ServerID  int
	PushName  string

	// Message type
	MessageType string

	// Text content
	TextContent string
	Caption     string

	// Media fields (for download)
	MediaURL          string
	MediaDirectPath   string
	MediaKey          []byte
	MediaKeyTimestamp int64
	FileSHA256        []byte
	FileEncSHA256     []byte
	FileLength        int64
	Mimetype          string

	// Media metadata
	Width               int
	Height              int
	DurationSeconds     int
	Thumbnail           []byte
	ThumbnailDirectPath string
	ThumbnailSHA256     []byte
	ThumbnailEncSHA256  []byte

	// Sticker specific
	IsAnimated bool

	// Audio specific
	IsPTT    bool
	Waveform []byte

	// Video specific
	IsGIF            bool
	StreamingSidecar []byte

	// Quote/Reply context
	QuotedMessageID   string
	QuotedSenderLID   types.JID
	QuotedMessageType string
	QuotedContent     string

	// Mentions
	MentionedJIDs []types.JID
	GroupMentions []GroupMention

	// Forwarding
	IsForwarded     bool
	ForwardingScore int

	// Location
	Latitude         float64
	Longitude        float64
	LocationName     string
	LocationAddress  string
	LocationURL      string
	IsLiveLocation   bool
	AccuracyMeters   int
	SpeedMPS         float64
	DegreesClockwise int
	LiveLocationSeq  int

	// Contact card
	VCard       string
	DisplayName string

	// Poll
	PollName          string
	PollOptions       []string
	PollSelectMax     int
	PollEncryptionKey []byte

	// Flags
	IsBroadcast      bool
	BroadcastListJID types.JID
	IsEphemeral      bool
	IsViewOnce       bool
	IsStarred        bool
	IsEdited         bool
	EditTimestamp    time.Time
	IsRevoked        bool

	// Protocol message
	ProtocolType int

	CreatedAt time.Time
}

// GroupMention represents a group mention in a message.
type GroupMention struct {
	GroupJID     types.JID `json:"group_jid"`
	GroupSubject string    `json:"group_subject,omitempty"`
}

// MessageStore handles message operations.
type MessageStore struct {
	store *Store
}

// NewMessageStore creates a new MessageStore.
func NewMessageStore(s *Store) *MessageStore {
	return &MessageStore{store: s}
}

// Put stores or updates a message.
func (s *MessageStore) Put(m *Message) error {
	now := time.Now().Unix()

	// Serialize arrays to JSON
	mentionedJIDs, _ := json.Marshal(jidsToStrings(m.MentionedJIDs))
	groupMentions, _ := json.Marshal(m.GroupMentions)
	pollOptions, _ := json.Marshal(m.PollOptions)

	var editTs sql.NullInt64
	if !m.EditTimestamp.IsZero() {
		editTs.Int64 = m.EditTimestamp.Unix()
		editTs.Valid = true
	}

	_, err := s.store.Exec(`
		INSERT INTO orion_messages (
			id, chat_jid, sender_lid, from_me, timestamp, server_id, push_name,
			message_type, text_content, caption,
			media_url, media_direct_path, media_key, media_key_timestamp,
			file_sha256, file_enc_sha256, file_length, mimetype,
			width, height, duration_seconds, thumbnail,
			thumbnail_direct_path, thumbnail_sha256, thumbnail_enc_sha256,
			is_animated,
			is_ptt, waveform, is_gif, streaming_sidecar,
			quoted_message_id, quoted_sender_lid, quoted_message_type, quoted_content,
			mentioned_jids, group_mentions,
			is_forwarded, forwarding_score,
			latitude, longitude, location_name, location_address, location_url,
			is_live_location, accuracy_meters, speed_mps, degrees_clockwise, live_location_sequence,
			vcard, display_name,
			poll_name, poll_options, poll_select_max, poll_encryption_key,
			is_broadcast, broadcast_list_jid, is_ephemeral, is_view_once,
			is_starred, is_edited, edit_timestamp, is_revoked,
			protocol_type, created_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?,
			?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?,
			?,
			?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?,
			?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?
		)
		ON CONFLICT(id, chat_jid) DO UPDATE SET
			text_content = COALESCE(excluded.text_content, orion_messages.text_content),
			caption = COALESCE(excluded.caption, orion_messages.caption),
			is_starred = excluded.is_starred,
			is_edited = excluded.is_edited,
			edit_timestamp = COALESCE(excluded.edit_timestamp, orion_messages.edit_timestamp),
			is_revoked = excluded.is_revoked
	`,
		m.ID, m.ChatJID.String(), nullJID(m.SenderLID), boolToInt(m.FromMe), m.Timestamp.Unix(), m.ServerID, nullString(m.PushName),
		m.MessageType, nullString(m.TextContent), nullString(m.Caption),
		nullString(m.MediaURL), nullString(m.MediaDirectPath), m.MediaKey, nullInt64(m.MediaKeyTimestamp),
		m.FileSHA256, m.FileEncSHA256, nullInt64(m.FileLength), nullString(m.Mimetype),
		nullInt(m.Width), nullInt(m.Height), nullInt(m.DurationSeconds), m.Thumbnail,
		nullString(m.ThumbnailDirectPath), m.ThumbnailSHA256, m.ThumbnailEncSHA256,
		boolToInt(m.IsAnimated),
		boolToInt(m.IsPTT), m.Waveform, boolToInt(m.IsGIF), m.StreamingSidecar,
		nullString(m.QuotedMessageID), nullJID(m.QuotedSenderLID), nullString(m.QuotedMessageType), nullString(m.QuotedContent),
		mentionedJIDs, groupMentions,
		boolToInt(m.IsForwarded), nullInt(m.ForwardingScore),
		nullFloat(m.Latitude), nullFloat(m.Longitude), nullString(m.LocationName), nullString(m.LocationAddress), nullString(m.LocationURL),
		boolToInt(m.IsLiveLocation), nullInt(m.AccuracyMeters), nullFloat(m.SpeedMPS), nullInt(m.DegreesClockwise), nullInt(m.LiveLocationSeq),
		nullString(m.VCard), nullString(m.DisplayName),
		nullString(m.PollName), pollOptions, nullInt(m.PollSelectMax), m.PollEncryptionKey,
		boolToInt(m.IsBroadcast), nullJID(m.BroadcastListJID), boolToInt(m.IsEphemeral), boolToInt(m.IsViewOnce),
		boolToInt(m.IsStarred), boolToInt(m.IsEdited), editTs, boolToInt(m.IsRevoked),
		nullInt(m.ProtocolType), now,
	)
	return err
}

// Get retrieves a message by ID and chat JID.
func (s *MessageStore) Get(id string, chatJID types.JID) (*Message, error) {
	row := s.store.QueryRow(`
		SELECT id, chat_jid, sender_lid, from_me, timestamp, server_id, push_name,
			message_type, text_content, caption,
			media_url, media_direct_path, media_key, media_key_timestamp,
			file_sha256, file_enc_sha256, file_length, mimetype,
			width, height, duration_seconds, thumbnail,
			quoted_message_id, quoted_sender_lid,
			mentioned_jids, is_forwarded, forwarding_score,
			is_ephemeral, is_view_once, is_starred, is_edited, edit_timestamp, is_revoked,
			created_at
		FROM orion_messages WHERE id = ? AND chat_jid = ?
	`, id, chatJID.String())

	return s.scanMessageBasic(row)
}

// GetByChat retrieves messages for a chat.
func (s *MessageStore) GetByChat(chatJID types.JID, limit, offset int) ([]*Message, error) {
	rows, err := s.store.Query(`
		SELECT id, chat_jid, sender_lid, from_me, timestamp, server_id, push_name,
			message_type, text_content, caption,
			media_url, media_direct_path, media_key, media_key_timestamp,
			file_sha256, file_enc_sha256, file_length, mimetype,
			width, height, duration_seconds, thumbnail,
			quoted_message_id, quoted_sender_lid,
			mentioned_jids, is_forwarded, forwarding_score,
			is_ephemeral, is_view_once, is_starred, is_edited, edit_timestamp, is_revoked,
			created_at
		FROM orion_messages WHERE chat_jid = ?
		ORDER BY timestamp DESC LIMIT ? OFFSET ?
	`, chatJID.String(), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanMessagesBasic(rows)
}

// Delete deletes a message.
func (s *MessageStore) Delete(id string, chatJID types.JID) error {
	_, err := s.store.Exec(`DELETE FROM orion_messages WHERE id = ? AND chat_jid = ?`, id, chatJID.String())
	return err
}

// SetStarred updates starred status.
func (s *MessageStore) SetStarred(id string, chatJID types.JID, starred bool) error {
	_, err := s.store.Exec(`UPDATE orion_messages SET is_starred = ? WHERE id = ? AND chat_jid = ?`,
		boolToInt(starred), id, chatJID.String())
	return err
}

// SetRevoked marks a message as revoked.
func (s *MessageStore) SetRevoked(id string, chatJID types.JID) error {
	_, err := s.store.Exec(`UPDATE orion_messages SET is_revoked = 1 WHERE id = ? AND chat_jid = ?`,
		id, chatJID.String())
	return err
}

// MarkEdited marks a message as edited.
func (s *MessageStore) MarkEdited(id string, chatJID types.JID, newContent string, editTime time.Time) error {
	_, err := s.store.Exec(`
		UPDATE orion_messages SET text_content = ?, is_edited = 1, edit_timestamp = ?
		WHERE id = ? AND chat_jid = ?
	`, newContent, editTime.Unix(), id, chatJID.String())
	return err
}

// GetMediaForDownload retrieves media fields needed for download.
func (s *MessageStore) GetMediaForDownload(id string, chatJID types.JID) (*MediaDownloadInfo, error) {
	row := s.store.QueryRow(`
		SELECT media_url, media_direct_path, media_key, file_sha256, file_enc_sha256, file_length, mimetype
		FROM orion_messages WHERE id = ? AND chat_jid = ?
	`, id, chatJID.String())

	var info MediaDownloadInfo
	var url, directPath, mimetype sql.NullString
	var fileLength sql.NullInt64

	err := row.Scan(&url, &directPath, &info.MediaKey, &info.FileSHA256, &info.FileEncSHA256, &fileLength, &mimetype)
	if err != nil {
		return nil, err
	}

	info.URL = url.String
	info.DirectPath = directPath.String
	info.FileLength = fileLength.Int64
	info.Mimetype = mimetype.String

	return &info, nil
}

// MediaDownloadInfo contains fields needed to download media.
type MediaDownloadInfo struct {
	URL           string
	DirectPath    string
	MediaKey      []byte
	FileSHA256    []byte
	FileEncSHA256 []byte
	FileLength    int64
	Mimetype      string
}

func (s *MessageStore) scanMessageBasic(row *sql.Row) (*Message, error) {
	var id, chatJIDStr, msgType string
	var senderLID, pushName, textContent, caption sql.NullString
	var mediaURL, mediaDirectPath, mimetype sql.NullString
	var quotedMsgID, quotedSenderLID sql.NullString
	var mentionedJIDsJSON sql.NullString
	var timestamp, createdAt int64
	var serverID, width, height, durationSecs, forwardingScore int
	var mediaKeyTs, fileLength sql.NullInt64
	var fromMe, isForwarded, isEphemeral, isViewOnce, isStarred, isEdited, isRevoked int
	var editTs sql.NullInt64
	var mediaKey, fileSHA, fileEncSHA, thumbnail []byte

	err := row.Scan(
		&id, &chatJIDStr, &senderLID, &fromMe, &timestamp, &serverID, &pushName,
		&msgType, &textContent, &caption,
		&mediaURL, &mediaDirectPath, &mediaKey, &mediaKeyTs,
		&fileSHA, &fileEncSHA, &fileLength, &mimetype,
		&width, &height, &durationSecs, &thumbnail,
		&quotedMsgID, &quotedSenderLID,
		&mentionedJIDsJSON, &isForwarded, &forwardingScore,
		&isEphemeral, &isViewOnce, &isStarred, &isEdited, &editTs, &isRevoked,
		&createdAt,
	)
	if err != nil {
		return nil, err
	}

	chatJID, _ := types.ParseJID(chatJIDStr)
	m := &Message{
		ID:              id,
		ChatJID:         chatJID,
		FromMe:          fromMe == 1,
		Timestamp:       time.Unix(timestamp, 0),
		ServerID:        serverID,
		PushName:        pushName.String,
		MessageType:     msgType,
		TextContent:     textContent.String,
		Caption:         caption.String,
		MediaURL:        mediaURL.String,
		MediaDirectPath: mediaDirectPath.String,
		MediaKey:        mediaKey,
		FileSHA256:      fileSHA,
		FileEncSHA256:   fileEncSHA,
		Mimetype:        mimetype.String,
		Width:           width,
		Height:          height,
		DurationSeconds: durationSecs,
		Thumbnail:       thumbnail,
		QuotedMessageID: quotedMsgID.String,
		IsForwarded:     isForwarded == 1,
		ForwardingScore: forwardingScore,
		IsEphemeral:     isEphemeral == 1,
		IsViewOnce:      isViewOnce == 1,
		IsStarred:       isStarred == 1,
		IsEdited:        isEdited == 1,
		IsRevoked:       isRevoked == 1,
		CreatedAt:       time.Unix(createdAt, 0),
	}

	if senderLID.Valid {
		m.SenderLID, _ = types.ParseJID(senderLID.String)
	}
	if quotedSenderLID.Valid {
		m.QuotedSenderLID, _ = types.ParseJID(quotedSenderLID.String)
	}
	if mediaKeyTs.Valid {
		m.MediaKeyTimestamp = mediaKeyTs.Int64
	}
	if fileLength.Valid {
		m.FileLength = fileLength.Int64
	}
	if editTs.Valid {
		m.EditTimestamp = time.Unix(editTs.Int64, 0)
	}
	if mentionedJIDsJSON.Valid {
		var jidStrs []string
		json.Unmarshal([]byte(mentionedJIDsJSON.String), &jidStrs)
		m.MentionedJIDs = stringsToJIDs(jidStrs)
	}

	return m, nil
}

func (s *MessageStore) scanMessagesBasic(rows *sql.Rows) ([]*Message, error) {
	var messages []*Message
	for rows.Next() {
		var id, chatJIDStr, msgType string
		var senderLID, pushName, textContent, caption sql.NullString
		var mediaURL, mediaDirectPath, mimetype sql.NullString
		var quotedMsgID, quotedSenderLID sql.NullString
		var mentionedJIDsJSON sql.NullString
		var timestamp, createdAt int64
		var serverID, width, height, durationSecs, forwardingScore int
		var mediaKeyTs, fileLength sql.NullInt64
		var fromMe, isForwarded, isEphemeral, isViewOnce, isStarred, isEdited, isRevoked int
		var editTs sql.NullInt64
		var mediaKey, fileSHA, fileEncSHA, thumbnail []byte

		err := rows.Scan(
			&id, &chatJIDStr, &senderLID, &fromMe, &timestamp, &serverID, &pushName,
			&msgType, &textContent, &caption,
			&mediaURL, &mediaDirectPath, &mediaKey, &mediaKeyTs,
			&fileSHA, &fileEncSHA, &fileLength, &mimetype,
			&width, &height, &durationSecs, &thumbnail,
			&quotedMsgID, &quotedSenderLID,
			&mentionedJIDsJSON, &isForwarded, &forwardingScore,
			&isEphemeral, &isViewOnce, &isStarred, &isEdited, &editTs, &isRevoked,
			&createdAt,
		)
		if err != nil {
			return nil, err
		}

		chatJID, _ := types.ParseJID(chatJIDStr)
		m := &Message{
			ID:              id,
			ChatJID:         chatJID,
			FromMe:          fromMe == 1,
			Timestamp:       time.Unix(timestamp, 0),
			ServerID:        serverID,
			PushName:        pushName.String,
			MessageType:     msgType,
			TextContent:     textContent.String,
			Caption:         caption.String,
			MediaURL:        mediaURL.String,
			MediaDirectPath: mediaDirectPath.String,
			MediaKey:        mediaKey,
			FileSHA256:      fileSHA,
			FileEncSHA256:   fileEncSHA,
			Mimetype:        mimetype.String,
			Width:           width,
			Height:          height,
			DurationSeconds: durationSecs,
			Thumbnail:       thumbnail,
			QuotedMessageID: quotedMsgID.String,
			IsForwarded:     isForwarded == 1,
			ForwardingScore: forwardingScore,
			IsEphemeral:     isEphemeral == 1,
			IsViewOnce:      isViewOnce == 1,
			IsStarred:       isStarred == 1,
			IsEdited:        isEdited == 1,
			IsRevoked:       isRevoked == 1,
			CreatedAt:       time.Unix(createdAt, 0),
		}

		if senderLID.Valid {
			m.SenderLID, _ = types.ParseJID(senderLID.String)
		}
		if quotedSenderLID.Valid {
			m.QuotedSenderLID, _ = types.ParseJID(quotedSenderLID.String)
		}
		if mediaKeyTs.Valid {
			m.MediaKeyTimestamp = mediaKeyTs.Int64
		}
		if fileLength.Valid {
			m.FileLength = fileLength.Int64
		}
		if editTs.Valid {
			m.EditTimestamp = time.Unix(editTs.Int64, 0)
		}

		messages = append(messages, m)
	}
	return messages, nil
}
