package store

import (
	"time"

	"go.mau.fi/whatsmeow/types"
)

// PrivacySettings represents privacy settings.
type PrivacySettings struct {
	GroupAdd     string
	LastSeen     string
	Status       string
	Profile      string
	ReadReceipts string
	Online       string
	CallAdd      string
	UpdatedAt    time.Time
}

// StatusPrivacyType represents a status privacy type configuration.
type StatusPrivacyType struct {
	Type      string
	IsDefault bool
}

// StatusPrivacyMember represents a member in a status privacy list.
type StatusPrivacyMember struct {
	Type string
	JID  types.JID
}

// PrivacyStore handles privacy settings operations.
type PrivacyStore struct {
	store *Store
}

// NewPrivacyStore creates a new PrivacyStore.
func NewPrivacyStore(s *Store) *PrivacyStore {
	return &PrivacyStore{store: s}
}

// Put saves privacy settings.
func (s *PrivacyStore) Put(settings *PrivacySettings) error {
	now := time.Now().Unix()
	_, err := s.store.Exec(`
		INSERT INTO orion_privacy_settings (id, group_add, last_seen, status, profile, read_receipts, online, call_add, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			group_add = excluded.group_add,
			last_seen = excluded.last_seen,
			status = excluded.status,
			profile = excluded.profile,
			read_receipts = excluded.read_receipts,
			online = excluded.online,
			call_add = excluded.call_add,
			updated_at = excluded.updated_at
	`, settings.GroupAdd, settings.LastSeen, settings.Status, settings.Profile,
		settings.ReadReceipts, settings.Online, settings.CallAdd, now)
	return err
}

// Get retrieves the current privacy settings.
func (s *PrivacyStore) Get() (*PrivacySettings, error) {
	var groupAdd, lastSeen, status, profile, readReceipts, online, callAdd string
	var updatedAt int64

	err := s.store.QueryRow(`
		SELECT group_add, last_seen, status, profile, read_receipts, online, call_add, updated_at
		FROM orion_privacy_settings WHERE id = 1
	`).Scan(&groupAdd, &lastSeen, &status, &profile, &readReceipts, &online, &callAdd, &updatedAt)

	if err != nil {
		return nil, err
	}

	return &PrivacySettings{
		GroupAdd:     groupAdd,
		LastSeen:     lastSeen,
		Status:       status,
		Profile:      profile,
		ReadReceipts: readReceipts,
		Online:       online,
		CallAdd:      callAdd,
		UpdatedAt:    time.Unix(updatedAt, 0),
	}, nil
}

// PutStatusPrivacyType saves a status privacy type configuration.
func (s *PrivacyStore) PutStatusPrivacyType(spt *StatusPrivacyType) error {
	now := time.Now().Unix()
	_, err := s.store.Exec(`
		INSERT INTO orion_status_privacy_types (type, is_default, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(type) DO UPDATE SET
			is_default = excluded.is_default,
			updated_at = excluded.updated_at
	`, spt.Type, boolToInt(spt.IsDefault), now)
	return err
}

// PutStatusPrivacyMember saves a status privacy list member.
func (s *PrivacyStore) PutStatusPrivacyMember(spm *StatusPrivacyMember) error {
	_, err := s.store.Exec(`
		INSERT INTO orion_status_privacy_members (type, jid)
		VALUES (?, ?)
		ON CONFLICT(type, jid) DO NOTHING
	`, spm.Type, spm.JID.String())
	return err
}
