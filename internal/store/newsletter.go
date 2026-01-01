package store

import (
	"database/sql"
	"time"

	"go.mau.fi/whatsmeow/types"
)

// Newsletter represents a newsletter/channel.
type Newsletter struct {
	JID               types.JID
	Name              string
	Description       string
	SubscriberCount   int64
	VerificationState string
	PictureID         string
	PictureURL        string
	PreviewURL        string
	InviteLink        string
	Role              string
	Muted             bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// NewsletterStore handles newsletter operations.
type NewsletterStore struct {
	store *Store
}

// NewNewsletterStore creates a new NewsletterStore.
func NewNewsletterStore(s *Store) *NewsletterStore {
	return &NewsletterStore{store: s}
}

// Put stores or updates a newsletter.
func (s *NewsletterStore) Put(n *Newsletter) error {
	now := time.Now().Unix()
	_, err := s.store.Exec(`
		INSERT INTO orion_newsletters (
			jid, name, description, subscriber_count, verification_state,
			picture_id, picture_url, preview_url, invite_link, role, muted,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(jid) DO UPDATE SET
			name = COALESCE(excluded.name, orion_newsletters.name),
			description = COALESCE(excluded.description, orion_newsletters.description),
			subscriber_count = COALESCE(excluded.subscriber_count, orion_newsletters.subscriber_count),
			verification_state = COALESCE(excluded.verification_state, orion_newsletters.verification_state),
			picture_id = COALESCE(excluded.picture_id, orion_newsletters.picture_id),
			picture_url = COALESCE(excluded.picture_url, orion_newsletters.picture_url),
			preview_url = COALESCE(excluded.preview_url, orion_newsletters.preview_url),
			invite_link = COALESCE(excluded.invite_link, orion_newsletters.invite_link),
			role = COALESCE(excluded.role, orion_newsletters.role),
			muted = excluded.muted,
			updated_at = excluded.updated_at
	`, n.JID.String(), n.Name, n.Description, n.SubscriberCount, n.VerificationState,
		n.PictureID, n.PictureURL, n.PreviewURL, n.InviteLink, n.Role, n.Muted, now, now)
	return err
}

// Get retrieves a newsletter by JID.
func (s *NewsletterStore) Get(jid types.JID) (*Newsletter, error) {
	var jidStr string
	var name, desc, verState, picID, picURL, prevURL, inviteLink, role sql.NullString
	var subCount sql.NullInt64
	var muted int
	var createdAt, updatedAt int64

	err := s.store.QueryRow(`
		SELECT jid, name, description, subscriber_count, verification_state,
			picture_id, picture_url, preview_url, invite_link, role, muted,
			created_at, updated_at
		FROM orion_newsletters WHERE jid = ?
	`, jid.String()).Scan(
		&jidStr, &name, &desc, &subCount, &verState,
		&picID, &picURL, &prevURL, &inviteLink, &role, &muted,
		&createdAt, &updatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	parsedJID, _ := types.ParseJID(jidStr)
	return &Newsletter{
		JID:               parsedJID,
		Name:              name.String,
		Description:       desc.String,
		SubscriberCount:   subCount.Int64,
		VerificationState: verState.String,
		PictureID:         picID.String,
		PictureURL:        picURL.String,
		PreviewURL:        prevURL.String,
		InviteLink:        inviteLink.String,
		Role:              role.String,
		Muted:             muted == 1,
		CreatedAt:         time.Unix(createdAt, 0),
		UpdatedAt:         time.Unix(updatedAt, 0),
	}, nil
}

// GetAll retrieves all newsletters.
func (s *NewsletterStore) GetAll() ([]*Newsletter, error) {
	rows, err := s.store.Query(`
		SELECT jid, name, description, subscriber_count, verification_state,
			picture_id, picture_url, preview_url, invite_link, role, muted,
			created_at, updated_at
		FROM orion_newsletters ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Newsletter
	for rows.Next() {
		var jidStr string
		var name, desc, verState, picID, picURL, prevURL, inviteLink, role sql.NullString
		var subCount sql.NullInt64
		var muted int
		var createdAt, updatedAt int64

		if err := rows.Scan(
			&jidStr, &name, &desc, &subCount, &verState,
			&picID, &picURL, &prevURL, &inviteLink, &role, &muted,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}

		parsedJID, _ := types.ParseJID(jidStr)
		result = append(result, &Newsletter{
			JID:               parsedJID,
			Name:              name.String,
			Description:       desc.String,
			SubscriberCount:   subCount.Int64,
			VerificationState: verState.String,
			PictureID:         picID.String,
			PictureURL:        picURL.String,
			PreviewURL:        prevURL.String,
			InviteLink:        inviteLink.String,
			Role:              role.String,
			Muted:             muted == 1,
			CreatedAt:         time.Unix(createdAt, 0),
			UpdatedAt:         time.Unix(updatedAt, 0),
		})
	}

	return result, rows.Err()
}

// Delete removes a newsletter.
func (s *NewsletterStore) Delete(jid types.JID) error {
	_, err := s.store.Exec(`DELETE FROM orion_newsletters WHERE jid = ?`, jid.String())
	return err
}

// Exists checks if a newsletter exists.
func (s *NewsletterStore) Exists(jid types.JID) (bool, error) {
	var count int
	err := s.store.QueryRow(`SELECT COUNT(*) FROM orion_newsletters WHERE jid = ?`, jid.String()).Scan(&count)
	return count > 0, err
}

// Count returns the total number of newsletters.
func (s *NewsletterStore) Count() (int, error) {
	var count int
	err := s.store.QueryRow(`SELECT COUNT(*) FROM orion_newsletters`).Scan(&count)
	return count, err
}

// UpdateProfilePic updates the newsletter's profile picture.
func (s *NewsletterStore) UpdateProfilePic(jid types.JID, picID, picURL string) error {
	now := time.Now().Unix()
	_, err := s.store.Exec(`
		UPDATE orion_newsletters SET picture_id = ?, picture_url = ?, updated_at = ? WHERE jid = ?
	`, picID, picURL, now, jid.String())
	return err
}
