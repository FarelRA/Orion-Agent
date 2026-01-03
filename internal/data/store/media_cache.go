package store

import (
	"time"

	"go.mau.fi/whatsmeow/types"
)

// MediaCache represents a cached media file.
type MediaCache struct {
	MessageID    string
	ChatJID      types.JID
	MediaType    string
	LocalPath    string
	DownloadedAt *time.Time
	FileSize     int64
}

// MediaCacheStore handles media cache operations.
type MediaCacheStore struct {
	store *Store
}

// NewMediaCacheStore creates a new MediaCacheStore.
func NewMediaCacheStore(s *Store) *MediaCacheStore {
	return &MediaCacheStore{store: s}
}

// Put stores or updates a media cache entry.
func (s *MediaCacheStore) Put(m *MediaCache) error {
	now := time.Now().Unix()

	_, err := s.store.Exec(`
		INSERT INTO orion_media_cache (message_id, chat_jid, media_type, local_path, downloaded_at, file_size)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(message_id, chat_jid) DO UPDATE SET
			local_path = excluded.local_path,
			downloaded_at = excluded.downloaded_at,
			file_size = excluded.file_size
	`, m.MessageID, m.ChatJID.String(), m.MediaType, m.LocalPath, now, m.FileSize)
	return err
}

// Get retrieves a media cache entry.
func (s *MediaCacheStore) Get(messageID string, chatJID types.JID) (*MediaCache, error) {
	row := s.store.QueryRow(`
		SELECT message_id, chat_jid, media_type, local_path, downloaded_at, file_size
		FROM orion_media_cache WHERE message_id = ? AND chat_jid = ?
	`, messageID, chatJID.String())

	var m MediaCache
	var chatJIDStr string
	var downloadedAt int64

	err := row.Scan(&m.MessageID, &chatJIDStr, &m.MediaType, &m.LocalPath, &downloadedAt, &m.FileSize)
	if err != nil {
		return nil, err
	}

	m.ChatJID, _ = types.ParseJID(chatJIDStr)
	if downloadedAt > 0 {
		t := time.Unix(downloadedAt, 0)
		m.DownloadedAt = &t
	}

	return &m, nil
}

// GetLocalPath returns the local file path for a message's media.
func (s *MediaCacheStore) GetLocalPath(messageID string, chatJID types.JID) (string, error) {
	var path string
	err := s.store.QueryRow(`
		SELECT local_path FROM orion_media_cache WHERE message_id = ? AND chat_jid = ?
	`, messageID, chatJID.String()).Scan(&path)
	return path, err
}

// Delete removes a media cache entry.
func (s *MediaCacheStore) Delete(messageID string, chatJID types.JID) error {
	_, err := s.store.Exec(`DELETE FROM orion_media_cache WHERE message_id = ? AND chat_jid = ?`,
		messageID, chatJID.String())
	return err
}

// GetByChat retrieves all cached media for a chat.
func (s *MediaCacheStore) GetByChat(chatJID types.JID) ([]*MediaCache, error) {
	rows, err := s.store.Query(`
		SELECT message_id, chat_jid, media_type, local_path, downloaded_at, file_size
		FROM orion_media_cache WHERE chat_jid = ?
		ORDER BY downloaded_at DESC
	`, chatJID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*MediaCache
	for rows.Next() {
		var m MediaCache
		var chatJIDStr string
		var downloadedAt int64

		if err := rows.Scan(&m.MessageID, &chatJIDStr, &m.MediaType, &m.LocalPath, &downloadedAt, &m.FileSize); err != nil {
			return nil, err
		}

		m.ChatJID, _ = types.ParseJID(chatJIDStr)
		if downloadedAt > 0 {
			t := time.Unix(downloadedAt, 0)
			m.DownloadedAt = &t
		}

		results = append(results, &m)
	}

	return results, nil
}
