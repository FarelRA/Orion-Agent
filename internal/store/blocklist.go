package store

import (
	"time"

	"go.mau.fi/whatsmeow/types"
)

// BlockedContact represents a blocked contact.
type BlockedContact struct {
	JID       types.JID
	BlockedAt time.Time
}

// BlocklistStore handles blocklist operations.
type BlocklistStore struct {
	store *Store
}

// NewBlocklistStore creates a new BlocklistStore.
func NewBlocklistStore(s *Store) *BlocklistStore {
	return &BlocklistStore{store: s}
}

// Put adds or updates a blocked contact.
func (s *BlocklistStore) Put(jid types.JID) error {
	now := time.Now().Unix()
	_, err := s.store.Exec(`
		INSERT INTO orion_blocklist (jid, blocked_at)
		VALUES (?, ?)
		ON CONFLICT(jid) DO UPDATE SET blocked_at = excluded.blocked_at
	`, jid.String(), now)
	return err
}

// PutMany saves multiple blocked contacts at once.
func (s *BlocklistStore) PutMany(jids []types.JID) error {
	if len(jids) == 0 {
		return nil
	}

	tx, err := s.store.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().Unix()
	stmt, err := tx.Prepare(`
		INSERT INTO orion_blocklist (jid, blocked_at)
		VALUES (?, ?)
		ON CONFLICT(jid) DO UPDATE SET blocked_at = excluded.blocked_at
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, jid := range jids {
		if _, err := stmt.Exec(jid.String(), now); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Remove removes a contact from the blocklist.
func (s *BlocklistStore) Remove(jid types.JID) error {
	_, err := s.store.Exec(`DELETE FROM orion_blocklist WHERE jid = ?`, jid.String())
	return err
}

// Replace replaces the entire blocklist with new JIDs.
func (s *BlocklistStore) Replace(jids []types.JID) error {
	tx, err := s.store.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear existing
	if _, err := tx.Exec(`DELETE FROM orion_blocklist`); err != nil {
		return err
	}

	// Insert new
	now := time.Now().Unix()
	stmt, err := tx.Prepare(`INSERT INTO orion_blocklist (jid, blocked_at) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, jid := range jids {
		if _, err := stmt.Exec(jid.String(), now); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetAll returns all blocked contacts.
func (s *BlocklistStore) GetAll() ([]BlockedContact, error) {
	rows, err := s.store.Query(`SELECT jid, blocked_at FROM orion_blocklist ORDER BY blocked_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []BlockedContact
	for rows.Next() {
		var jidStr string
		var blockedAt int64
		if err := rows.Scan(&jidStr, &blockedAt); err != nil {
			return nil, err
		}
		jid, _ := types.ParseJID(jidStr)
		result = append(result, BlockedContact{
			JID:       jid,
			BlockedAt: time.Unix(blockedAt, 0),
		})
	}

	return result, rows.Err()
}

// IsBlocked checks if a JID is blocked.
func (s *BlocklistStore) IsBlocked(jid types.JID) (bool, error) {
	var count int
	err := s.store.QueryRow(`SELECT COUNT(*) FROM orion_blocklist WHERE jid = ?`, jid.String()).Scan(&count)
	return count > 0, err
}

// Count returns the number of blocked contacts.
func (s *BlocklistStore) Count() (int, error) {
	var count int
	err := s.store.QueryRow(`SELECT COUNT(*) FROM orion_blocklist`).Scan(&count)
	return count, err
}
